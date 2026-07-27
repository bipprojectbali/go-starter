package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// members.go — kelola ANGGOTA workspace aktif (model membership). Dipisah dari
// workspace.go (pengaturan nama) agar tiap file di bawah batas handler 150 baris.

// canManageMembers melaporkan apakah aktor boleh mengelola anggota workspace:
// owner/admin tenant, atau platform (super_admin/staff yang membantu).
func canManageMembers(ctx context.Context) bool {
	if session.IsRoot(ctx) {
		return true
	}
	role := session.Role(ctx)
	return role == authz.RoleNameOwner || role == authz.RoleNameAdmin || isPlatformRole(role)
}

// MembersPage — GET /admin/members. Daftar anggota + undangan pending.
func (h *Handler) MembersPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := session.TenantID(ctx)
	rows, err := h.q(ctx).ListMembersByTenant(ctx, tenantID)
	if err != nil {
		h.Log.Error("members: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	members := make([]panel.MemberRow, 0, len(rows))
	for _, m := range rows {
		avatar := ""
		if m.AvatarUrl != nil {
			avatar = *m.AvatarUrl
		}
		members = append(members, panel.MemberRow{
			UserID: m.UserID, Email: m.Email, Role: m.Role,
			AvatarURL: avatar, Status: m.Status,
		})
	}
	// Undangan pending (fail-soft: gagal → daftar kosong, halaman tetap tampil).
	invites := []panel.InviteRow{}
	if inv, e := h.q(ctx).ListInvitesByTenant(ctx, tenantID); e == nil {
		for _, i := range inv {
			invites = append(invites, panel.InviteRow{
				ID: i.ID, Email: i.Email, Role: i.Role,
				Link: inviteLink(r, i.Token), Expires: fmtLocal(i.ExpiresAt),
			})
		}
	} else {
		h.Log.Error("members: list invites", "err", e)
	}

	h.renderShell(w, r, "Anggota", "go_starter /admin", "/admin/members", adminNav,
		panel.Members(members, invites, canManageMembers(ctx), session.UserID(ctx),
			workspaceErrMsg(r.URL.Query().Get("err"))))
}

// MemberSetRole — POST /admin/members/{id}/role. Ubah role anggota di workspace
// AKTIF. Owner/admin saja; owner terakhir tak boleh diturunkan (workspace yatim).
func (h *Handler) MemberSetRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageMembers(ctx) {
		http.Redirect(w, r, "/admin/members?err=forbidden", http.StatusSeeOther)
		return
	}
	targetID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	newRole := r.FormValue("role")
	if !authz.ValidRoleName(newRole) {
		http.Redirect(w, r, "/admin/members?err=role", http.StatusSeeOther)
		return
	}
	tenantID := session.TenantID(ctx)
	actor, target, err := h.loadActorTarget(ctx, targetID, tenantID)
	if err != nil {
		http.Redirect(w, r, "/admin/members?err=notfound", http.StatusSeeOther)
		return
	}
	if err := authz.GuardSetRole(actor, target, authz.ParseRole(newRole)); err != nil {
		http.Redirect(w, r, "/admin/members?err=forbidden", http.StatusSeeOther)
		return
	}
	if target.Role == authz.RoleOwner && newRole != authz.RoleNameOwner {
		if n, e := h.q(ctx).CountTenantOwners(ctx, tenantID); e == nil && n <= 1 {
			http.Redirect(w, r, "/admin/members?err=lastowner", http.StatusSeeOther)
			return
		}
	}
	if err := h.q(ctx).UpdateMemberRole(ctx, db.UpdateMemberRoleParams{
		UserID: targetID, TenantID: tenantID, Role: newRole,
	}); err != nil {
		h.Log.Error("members: update role", "err", err)
		http.Redirect(w, r, "/admin/members?err=failed", http.StatusSeeOther)
		return
	}
	h.audit(ctx, actor.ID, "member.role.update", targetID, map[string]string{"to": newRole})
	http.Redirect(w, r, "/admin/members", http.StatusSeeOther)
}

// MemberRemove — POST /admin/members/{id}/remove. Keluarkan anggota dari
// workspace aktif (membership dihapus; USER-nya tetap ada — identitas global).
func (h *Handler) MemberRemove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageMembers(ctx) {
		http.Redirect(w, r, "/admin/members?err=forbidden", http.StatusSeeOther)
		return
	}
	targetID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	tenantID := session.TenantID(ctx)
	actor, target, err := h.loadActorTarget(ctx, targetID, tenantID)
	if err != nil {
		http.Redirect(w, r, "/admin/members?err=notfound", http.StatusSeeOther)
		return
	}
	if err := authz.GuardDelete(actor, target); err != nil {
		http.Redirect(w, r, "/admin/members?err=forbidden", http.StatusSeeOther)
		return
	}
	if target.Role == authz.RoleOwner {
		if n, e := h.q(ctx).CountTenantOwners(ctx, tenantID); e == nil && n <= 1 {
			http.Redirect(w, r, "/admin/members?err=lastowner", http.StatusSeeOther)
			return
		}
	}
	if err := h.q(ctx).DeleteMembership(ctx, db.DeleteMembershipParams{
		UserID: targetID, TenantID: tenantID,
	}); err != nil {
		h.Log.Error("members: remove", "err", err)
		http.Redirect(w, r, "/admin/members?err=failed", http.StatusSeeOther)
		return
	}
	h.audit(ctx, actor.ID, "member.remove", targetID, nil)
	http.Redirect(w, r, "/admin/members", http.StatusSeeOther)
}
