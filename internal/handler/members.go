package handler

import (
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// members.go — AKSI atas anggota workspace aktif: ubah role & keluarkan.
// Halaman daftarnya ada di members_page.go (aturan apa yang boleh DILIHAT tumbuh
// bersama kebijakan privasi, bukan bersama daftar aksi).

// MemberSetRole — POST /w/{workspace}/members/{id}/role. Ubah role anggota di workspace
// AKTIF. Owner/admin saja; owner terakhir tak boleh diturunkan (workspace yatim).
func (h *Handler) MemberSetRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageMembers(ctx) {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	targetID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	newRole := r.FormValue("role")
	if !authz.ValidRoleName(newRole) {
		wsRedirect(w, r, "/members", "role")
		return
	}
	tenantID := session.TenantID(ctx)
	actor, target, err := h.loadActorTarget(ctx, targetID, tenantID)
	if err != nil {
		wsRedirect(w, r, "/members", "notfound")
		return
	}
	if err := authz.GuardSetRole(actor, target, authz.ParseRole(newRole)); err != nil {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	if target.Role == authz.RoleOwner && newRole != authz.RoleNameOwner {
		if n, e := h.q(ctx).CountTenantOwners(ctx, tenantID); e == nil && n <= 1 {
			wsRedirect(w, r, "/members", "lastowner")
			return
		}
	}
	if err := h.q(ctx).UpdateMemberRole(ctx, db.UpdateMemberRoleParams{
		UserID: targetID, TenantID: tenantID, Role: newRole,
	}); err != nil {
		h.Log.Error("members: update role", "err", err)
		wsRedirect(w, r, "/members", "failed")
		return
	}
	h.audit(ctx, actor.ID, "member.role.update", targetID, map[string]string{"to": newRole})
	// Beri tahu yang bersangkutan — perubahan role mengubah apa yang bisa ia
	// lakukan, jadi ia berhak tahu tanpa harus menyadarinya sendiri.
	h.notify(ctx, targetID, tenantID, "member.role.changed", notifPayload{Role: newRole})
	wsRedirect(w, r, "/members", "")
}

// MemberRemove — POST /w/{workspace}/members/{id}/remove. Keluarkan anggota dari
// workspace aktif (membership dihapus; USER-nya tetap ada — identitas global).
func (h *Handler) MemberRemove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageMembers(ctx) {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	targetID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	tenantID := session.TenantID(ctx)
	actor, target, err := h.loadActorTarget(ctx, targetID, tenantID)
	if err != nil {
		wsRedirect(w, r, "/members", "notfound")
		return
	}
	if err := authz.GuardDelete(actor, target); err != nil {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	if target.Role == authz.RoleOwner {
		if n, e := h.q(ctx).CountTenantOwners(ctx, tenantID); e == nil && n <= 1 {
			wsRedirect(w, r, "/members", "lastowner")
			return
		}
	}
	if err := h.q(ctx).DeleteMembership(ctx, db.DeleteMembershipParams{
		UserID: targetID, TenantID: tenantID,
	}); err != nil {
		h.Log.Error("members: remove", "err", err)
		wsRedirect(w, r, "/members", "failed")
		return
	}
	h.audit(ctx, actor.ID, "member.remove", targetID, nil)
	// Notifikasi ditulis SETELAH membership dihapus — sengaja: baris notifikasi
	// hanya ber-FK ke tenants (bukan memberships), jadi tetap ada & terbaca oleh
	// mantan anggota yang sudah tak punya akses ke workspace itu.
	h.notify(ctx, targetID, tenantID, "member.removed", notifPayload{})
	wsRedirect(w, r, "/members", "")
}
