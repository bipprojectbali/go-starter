package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"go_stater/internal/authz"
	"go_stater/internal/db"
	"go_stater/internal/session"
	"go_stater/internal/ui/pages/dev"

	"github.com/go-chi/chi/v5"
)

// DevUsersList — GET /dev/users. Daftar user (keyset) untuk panel developer.
func (h *Handler) DevUsersList(w http.ResponseWriter, r *http.Request) {
	cursorAt, cursorID := firstPageCursor()
	users, err := h.DB.ListUsers(r.Context(), db.ListUsersParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		PageSize:        pageSize,
	})
	if err != nil {
		h.Log.Error("dev users: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Aktor untuk menentukan kontrol mana yang boleh dirender (precompute).
	actorRole := authz.ParseRole(session.Role(r.Context()))
	canManageSuper := session.IsRoot(r.Context()) || actorRole >= authz.RoleSuperAdmin
	h.renderShell(w, r, "Users", "/dev/users",
		dev.UsersPage(toUserRows(users), canManageSuper))
}

// DevUserSetRole — POST /dev/users/{id}/role. Ubah role (di-guard + audit).
func (h *Handler) DevUserSetRole(w http.ResponseWriter, r *http.Request) {
	targetID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	newRole := r.FormValue("role")
	if !authz.ValidRoleName(newRole) {
		http.Error(w, "role tidak valid", http.StatusBadRequest)
		return
	}
	actor, target, err := h.loadActorTarget(r.Context(), targetID)
	if err != nil {
		h.devUserError(w, err)
		return
	}
	if err := authz.GuardSetRole(actor, target, authz.ParseRole(newRole)); err != nil {
		h.devUserError(w, err)
		return
	}
	if err := h.DB.UpdateUserRole(r.Context(), db.UpdateUserRoleParams{ID: targetID, Role: newRole}); err != nil {
		h.Log.Error("dev users: update role", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.audit(r.Context(), actor.ID, "user.role.update", targetID, map[string]string{"to": newRole})
	http.Redirect(w, r, "/dev/users", http.StatusSeeOther)
}

// DevUserSetStatus — POST /dev/users/{id}/status. Disable/block/aktifkan.
func (h *Handler) DevUserSetStatus(w http.ResponseWriter, r *http.Request) {
	targetID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	newStatus := r.FormValue("status")
	switch newStatus {
	case "active", "disabled", "blocked":
	default:
		http.Error(w, "status tidak valid", http.StatusBadRequest)
		return
	}
	actor, target, err := h.loadActorTarget(r.Context(), targetID)
	if err != nil {
		h.devUserError(w, err)
		return
	}
	if err := authz.GuardMutateStatus(actor, target, newStatus); err != nil {
		h.devUserError(w, err)
		return
	}
	if err := h.DB.UpdateUserStatus(r.Context(), db.UpdateUserStatusParams{ID: targetID, Status: newStatus}); err != nil {
		h.Log.Error("dev users: update status", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.audit(r.Context(), actor.ID, "user.status.update", targetID, map[string]string{"to": newStatus})
	http.Redirect(w, r, "/dev/users", http.StatusSeeOther)
}

// DevUserDelete — DELETE /dev/users/{id}. Soft-delete (di-guard + audit).
func (h *Handler) DevUserDelete(w http.ResponseWriter, r *http.Request) {
	targetID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	actor, target, err := h.loadActorTarget(r.Context(), targetID)
	if err != nil {
		h.devUserError(w, err)
		return
	}
	if err := authz.GuardDelete(actor, target); err != nil {
		h.devUserError(w, err)
		return
	}
	if err := h.DB.SoftDeleteUser(r.Context(), targetID); err != nil {
		h.Log.Error("dev users: delete", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.audit(r.Context(), actor.ID, "user.delete", targetID, nil)
	http.Redirect(w, r, "/dev/users", http.StatusSeeOther)
}

// --- helper ---

func (h *Handler) parseTargetID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "id tidak valid", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

// loadActorTarget membangun Actor (dari session) & Target (dari DB) untuk guard.
func (h *Handler) loadActorTarget(ctx context.Context, targetID int64) (authz.Actor, authz.Target, error) {
	actor := authz.Actor{
		ID:     session.UserID(ctx),
		Role:   authz.ParseRole(session.Role(ctx)),
		IsRoot: session.IsRoot(ctx),
	}
	tu, err := h.DB.GetUser(ctx, targetID)
	if err != nil {
		return actor, authz.Target{}, err
	}
	target := authz.Target{
		ID:          tu.ID,
		Role:        authz.ParseRole(tu.Role),
		IsEnvSuperA: isSuperAdminEmail(tu.Email),
	}
	return actor, target, nil
}

// devUserError memetakan error guard ke status HTTP yang sesuai.
func (h *Handler) devUserError(w http.ResponseWriter, err error) {
	switch err {
	case authz.ErrProtectedRoot, authz.ErrForbidden, authz.ErrSelfLockout:
		http.Error(w, err.Error(), http.StatusForbidden)
	case authz.ErrInvalidRole:
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		h.Log.Error("dev users: guard/load", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// audit menulis jejak aksi admin (metadata TANPA PII — id saja).
func (h *Handler) audit(ctx context.Context, actorID int64, action string, targetID int64, meta map[string]string) {
	raw := []byte("{}")
	if meta != nil {
		if b, err := json.Marshal(meta); err == nil {
			raw = b
		}
	}
	if _, err := h.DB.CreateAuditLog(ctx, db.CreateAuditLogParams{
		ActorUserID: &actorID,
		Action:      action,
		TargetType:  "user",
		TargetID:    &targetID,
		Metadata:    raw,
	}); err != nil {
		h.Log.Error("audit log", "action", action, "err", err) // jangan gagalkan aksi utama
	}
}

// toUserRows memetakan db.User → dev.UserRow (view), menandai root env.
func toUserRows(users []db.User) []dev.UserRow {
	rows := make([]dev.UserRow, 0, len(users))
	for _, u := range users {
		avatar := ""
		if u.AvatarUrl != nil {
			avatar = *u.AvatarUrl
		}
		rows = append(rows, dev.UserRow{
			ID:        u.ID,
			Email:     u.Email,
			Role:      u.Role,
			Status:    u.Status,
			AvatarURL: avatar,
			IsRoot:    isSuperAdminEmail(u.Email),
		})
	}
	return rows
}
