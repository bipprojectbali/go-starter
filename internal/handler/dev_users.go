package handler

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/dev"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/starfederation/datastar-go/datastar"
)

// firstPageCursor mengembalikan cursor keyset untuk halaman pertama:
// (created_at, id) = maksimum, sehingga semua baris lolos syarat < cursor.
func firstPageCursor() (pgtype.Timestamptz, int64) {
	return pgtype.Timestamptz{Valid: true, InfinityModifier: pgtype.Infinity}, math.MaxInt64
}

// DevUsersList — GET /dev/users. Daftar user (keyset) untuk panel developer.
func (h *Handler) DevUsersList(w http.ResponseWriter, r *http.Request) {
	cursorAt, cursorID := firstPageCursor()
	users, err := h.q(r.Context()).ListUsers(r.Context(), db.ListUsersParams{
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
	byUser := h.membershipsByUser(r.Context(), users) // satu query batch (anti N+1)
	h.renderShell(w, r, "Users", "go_starter /dev", "/dev/users", devNav(),
		dev.UsersPage(toUserRows(users, byUser), canManageSuper))
}

// DevUserSetRole — POST /dev/users/{id}/role. Ubah role (di-guard + audit).
func (h *Handler) DevUserSetRole(w http.ResponseWriter, r *http.Request) {
	targetID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	newRole := r.FormValue("role")
	if !authz.ValidRoleName(newRole) {
		h.devFlash(w, r, false, "Role tidak valid")
		return
	}
	// Role kini PER-WORKSPACE → butuh tenant mana. Form mengirimnya (panel /dev
	// menampilkan satu baris per membership).
	tenantID, err := strconv.ParseInt(r.FormValue("tenant"), 10, 64)
	if err != nil || tenantID == 0 {
		h.devFlash(w, r, false, "Workspace tidak valid")
		return
	}
	actor, target, err := h.loadActorTarget(r.Context(), targetID, tenantID)
	if err != nil {
		h.devFlashErr(w, r, err)
		return
	}
	if err := authz.GuardSetRole(actor, target, authz.ParseRole(newRole)); err != nil {
		h.devFlashErr(w, r, err)
		return
	}
	// Cegah workspace YATIM: menurunkan owner terakhir → ditolak.
	if target.Role == authz.RoleOwner && newRole != authz.RoleNameOwner {
		if n, e := h.q(r.Context()).CountTenantOwners(r.Context(), tenantID); e == nil && n <= 1 {
			h.devFlash(w, r, false, "Tak bisa menurunkan owner terakhir workspace")
			return
		}
	}
	if err := h.q(r.Context()).UpdateMemberRole(r.Context(), db.UpdateMemberRoleParams{
		UserID: targetID, TenantID: tenantID, Role: newRole,
	}); err != nil {
		h.Log.Error("dev users: update role", "err", err)
		h.devFlash(w, r, false, "Gagal menyimpan perubahan")
		return
	}
	h.audit(r.Context(), actor.ID, "member.role.update", targetID, map[string]string{"to": newRole})
	h.devRowUpdated(w, r, targetID, "Role diubah ke "+newRole)
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
		h.devFlash(w, r, false, "Status tidak valid")
		return
	}
	actor, target, err := h.loadActorTarget(r.Context(), targetID, session.TenantID(r.Context()))
	if err != nil {
		h.devFlashErr(w, r, err)
		return
	}
	if err := authz.GuardMutateStatus(actor, target, newStatus); err != nil {
		h.devFlashErr(w, r, err)
		return
	}
	if err := h.q(r.Context()).UpdateUserStatus(r.Context(), db.UpdateUserStatusParams{ID: targetID, Status: newStatus}); err != nil {
		h.Log.Error("dev users: update status", "err", err)
		h.devFlash(w, r, false, "Gagal menyimpan perubahan")
		return
	}
	h.audit(r.Context(), actor.ID, "user.status.update", targetID, map[string]string{"to": newStatus})
	h.devRowUpdated(w, r, targetID, "Status diubah ke "+newStatus)
}

// DevUserDelete — POST /dev/users/{id}/delete. Soft-delete (di-guard + audit).
func (h *Handler) DevUserDelete(w http.ResponseWriter, r *http.Request) {
	targetID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	actor, target, err := h.loadActorTarget(r.Context(), targetID, session.TenantID(r.Context()))
	if err != nil {
		h.devFlashErr(w, r, err)
		return
	}
	if err := authz.GuardDelete(actor, target); err != nil {
		h.devFlashErr(w, r, err)
		return
	}
	if err := h.q(r.Context()).SoftDeleteUser(r.Context(), targetID); err != nil {
		h.Log.Error("dev users: delete", "err", err)
		h.devFlash(w, r, false, "Gagal menghapus user")
		return
	}
	h.audit(r.Context(), actor.ID, "user.delete", targetID, nil)
	// Hapus baris dari tabel + flash sukses.
	sse := datastar.NewSSE(w, r)
	_ = sse.RemoveElement("#user-" + strconv.FormatInt(targetID, 10))
	patchFlash(sse, true, "User dihapus")
}

// --- helper flash / SSE ---

// devRowUpdated me-render ulang baris user (kontrol menampilkan nilai baru) +
// flash sukses. Ini yang menggantikan reload penuh: perubahan langsung terlihat.
func (h *Handler) devRowUpdated(w http.ResponseWriter, r *http.Request, targetID int64, msg string) {
	u, err := h.q(r.Context()).GetUser(r.Context(), targetID)
	if err != nil {
		h.Log.Error("dev users: reload row", "err", err)
		h.devFlash(w, r, false, "Tersimpan, tapi gagal memuat ulang baris")
		return
	}
	canManageSuper := session.IsRoot(r.Context()) ||
		authz.ParseRole(session.Role(r.Context())) >= authz.RoleSuperAdmin
	one := []db.User{u}
	row := toUserRows(one, h.membershipsByUser(r.Context(), one))[0]

	sse := datastar.NewSSE(w, r)
	var sb strings.Builder
	if err := dev.UserRowNode(row, canManageSuper).Render(&sb); err != nil {
		h.Log.Error("dev users: render row", "err", err)
		return
	}
	// Ganti baris berdasarkan id (mode default outer, morph by id).
	_ = sse.PatchElements(sb.String())
	patchFlash(sse, true, msg)
}

// devFlash mengirim hanya toast (tanpa mengubah baris).
func (h *Handler) devFlash(w http.ResponseWriter, r *http.Request, ok bool, msg string) {
	sse := datastar.NewSSE(w, r)
	patchFlash(sse, ok, msg)
}

// devFlashErr memetakan error guard ke pesan toast yang ramah.
func (h *Handler) devFlashErr(w http.ResponseWriter, r *http.Request, err error) {
	msg := "Aksi ditolak"
	switch err {
	case authz.ErrProtectedRoot:
		msg = "User ini root (env) — tak bisa diubah"
	case authz.ErrSelfLockout:
		msg = "Tidak bisa melakukan aksi ini pada akun sendiri"
	case authz.ErrForbidden:
		msg = "Anda tidak berwenang untuk aksi ini"
	default:
		h.Log.Error("dev users: guard/load", "err", err)
		msg = "Terjadi kesalahan"
	}
	h.devFlash(w, r, false, msg)
}

// patchFlash mem-patch toast notifikasi (id "flash") via SSE. Toast auto-hilang
// lewat skrip kecil di komponen. ok=true → hijau, false → merah.
func patchFlash(sse *datastar.ServerSentEventGenerator, ok bool, msg string) {
	var sb strings.Builder
	if err := dev.Flash(ok, msg).Render(&sb); err == nil {
		_ = sse.PatchElements(sb.String())
	}
}

// --- helper umum ---

func (h *Handler) parseTargetID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "id tidak valid", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

// loadActorTarget membangun Actor (dari session) & Target (dari DB) untuk guard.
// tenantID = workspace tempat role target dinilai (role kini per-workspace).
func (h *Handler) loadActorTarget(ctx context.Context, targetID, tenantID int64) (authz.Actor, authz.Target, error) {
	actor := authz.Actor{
		ID:     session.UserID(ctx),
		Role:   authz.ParseRole(session.Role(ctx)),
		IsRoot: session.IsRoot(ctx),
	}
	tu, err := h.q(ctx).GetUser(ctx, targetID)
	if err != nil {
		return actor, authz.Target{}, err
	}
	// Role target = role di WORKSPACE tsb (kini per-workspace, bukan properti user).
	// Tak punya membership di sana → otoritas terendah (member), bukan naik.
	role := authz.RoleMember
	if m, e := h.q(ctx).GetMembership(ctx, db.GetMembershipParams{UserID: targetID, TenantID: tenantID}); e == nil {
		role = authz.ParseRole(m.Role)
	}
	target := authz.Target{
		ID:          tu.ID,
		Role:        role,
		IsEnvSuperA: isSuperAdminEmail(tu.Email),
	}
	return actor, target, nil
}

// audit menulis jejak aksi admin atas USER lain (metadata TANPA PII — id saja).
func (h *Handler) audit(ctx context.Context, actorID int64, action string, targetID int64, meta map[string]string) {
	h.auditLog(ctx, actorID, action, "user", targetID, meta)
}

// auditLog menulis satu baris audit dengan target_type eksplisit. Dasar untuk
// audit() (target_type="user") & event auth (target_type="session"). Error
// di-log, TAK menggagalkan aksi utama. metadata TANPA PII (id/method saja).
func (h *Handler) auditLog(ctx context.Context, actorID int64, action, targetType string, targetID int64, meta map[string]string) {
	raw := []byte("{}")
	if meta != nil {
		if b, err := json.Marshal(meta); err == nil {
			raw = b
		}
	}
	// Audit di tx WithSuper TERPISAH (bukan Scope tx) — fail-soft struktural:
	// kegagalan tulis jejak TAK mengabort aksi utama (tx berbeda). tenant_id =
	// tenant aktor (selalu >0; semua user punya tenant), jadi jejak ter-atribusi
	// ke home-tenant aktor walau aksinya lintas-tenant (platform). Bypass RLS
	// perlu karena aktor platform menulis audit untuk target di tenant lain.
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		_, e := q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorUserID: &actorID,
			Action:      action,
			TargetType:  targetType,
			TargetID:    &targetID,
			Metadata:    raw,
			TenantID:    session.TenantID(ctx),
		})
		return e
	})
	if err != nil {
		h.Log.Error("audit log", "action", action, "err", err) // jangan gagalkan aksi utama
	}
}

// auditAuth mencatat event autentikasi (login/logout) — actor = user sendiri,
// target_type = "session". Fail-soft (via auditLog).
func (h *Handler) auditAuth(ctx context.Context, userID int64, action, method string) {
	var meta map[string]string
	if method != "" {
		meta = map[string]string{"method": method}
	}
	h.auditLog(ctx, userID, action, "session", userID, meta)
}

// toUserRows memetakan db.User → dev.UserRow (view), menandai root env.
// toUserRows memetakan db.User + membership-nya → dev.UserRow (view). Role kini
// PER-WORKSPACE: tiap user membawa daftar workspace tempat ia jadi anggota, jadi
// panel platform bisa mengubah role di workspace mana pun. byUser = hasil
// ListMembershipsForUsers (satu query batch — hindari N+1, Rule 13).
func toUserRows(users []db.User, byUser map[int64][]dev.WorkspaceRole) []dev.UserRow {
	rows := make([]dev.UserRow, 0, len(users))
	for _, u := range users {
		avatar := ""
		if u.AvatarUrl != nil {
			avatar = *u.AvatarUrl
		}
		isRoot := isSuperAdminEmail(u.Email)
		rows = append(rows, dev.UserRow{
			ID:         u.ID,
			Email:      u.Email,
			Status:     u.Status,
			AvatarURL:  avatar,
			IsRoot:     isRoot,
			Workspaces: byUser[u.ID],
			Quota:      int(u.WorkspaceQuota),
		})
	}
	return rows
}

// membershipsByUser menjalankan SATU query batch untuk semua user lalu
// mengelompokkannya per user_id (hindari N+1). Error → map kosong (fail-soft:
// tabel tetap tampil, kolom workspace saja yang kosong).
func (h *Handler) membershipsByUser(ctx context.Context, users []db.User) map[int64][]dev.WorkspaceRole {
	ids := make([]int64, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	out := make(map[int64][]dev.WorkspaceRole, len(users))
	if len(ids) == 0 {
		return out
	}
	rows, err := h.q(ctx).ListMembershipsForUsers(ctx, ids)
	if err != nil {
		h.Log.Error("dev users: list memberships", "err", err)
		return out
	}
	for _, m := range rows {
		out[m.UserID] = append(out[m.UserID], dev.WorkspaceRole{
			TenantID: m.TenantID, Name: m.Name, Role: m.Role,
		})
	}
	return out
}
