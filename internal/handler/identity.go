package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// RefreshIdentity me-load identitas user SEGAR dari DB tiap request (untuk user
// login) dan menyegarkan session. Ini membuat otorisasi real-time & self-healing:
//
//   - Role/status berubah di DB → langsung berlaku (tak tunggu re-login).
//   - User di-block/disable → logout paksa seketika.
//   - User di-soft-delete → session dibersihkan.
//   - Session lama tanpa role (shape berubah saat dev) → terisi ulang dari DB.
//
// Super-admin env (root) tetap kebal gate status. No-op untuk anonim.
// Pasang pada route yang butuh identitas akurat (Home + grup terproteksi),
// bukan pada static/health (hindari DB hit tak perlu).
func (h *Handler) RefreshIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		uid := session.UserID(ctx)
		if uid == 0 {
			next.ServeHTTP(w, r) // anonim — tak ada yang perlu disegarkan
			return
		}
		u, err := h.q(ctx).GetUser(ctx, uid)
		if err != nil {
			// User tak ada (dihapus) — GetUser memfilter deleted_at. Session basi.
			_ = session.Clear(ctx)
			next.ServeHTTP(w, r)
			return
		}

		isRoot := isSuperAdminEmail(u.Email)
		// Gate status real-time — root env kebal (tak bisa dikunci).
		if !isRoot && (u.Status == "blocked" || u.Status == "disabled") {
			_ = session.Clear(ctx)
			http.Redirect(w, r, "/login?err=inactive", http.StatusSeeOther)
			return
		}

		// Workspace aktif sudah DIVALIDASI middleware Scope (anggota terverifikasi).
		tenantID := session.TenantID(ctx)
		role := h.resolveRole(ctx, h.q(ctx), u, tenantID)
		avatar := ""
		if u.AvatarUrl != nil {
			avatar = *u.AvatarUrl
		}
		// Nama workspace segar dari DB (real-time: ganti nama langsung terlihat di
		// brand sidebar). tenants TANPA RLS-policy → terbaca di tx ber-scope. Fail-
		// soft: error → pakai nama tercache (jangan kosongkan brand karena glitch).
		// Slug ikut disegarkan: setiap URL workspace dibentuk darinya (0004), jadi
		// session tanpa slug (mis. dibuat sebelum 0004) akan sembuh sendiri di sini.
		tenantName, tenantSlug := session.TenantName(ctx), session.TenantSlug(ctx)
		if t, err := h.q(ctx).GetTenant(ctx, tenantID); err == nil {
			tenantName, tenantSlug = t.Name, t.Slug
		}
		// Tulis session hanya bila ada yang berubah (hindari commit tiap request).
		if session.Role(ctx) != role || session.IsRoot(ctx) != isRoot ||
			session.Email(ctx) != u.Email || session.AvatarURL(ctx) != avatar ||
			session.TenantName(ctx) != tenantName || session.TenantSlug(ctx) != tenantSlug {
			session.SetIdentity(ctx, u.ID, u.Email, role, isRoot, tenantID, tenantName, tenantSlug, avatar)
		}
		next.ServeHTTP(w, r)
	})
}

// resolveRole menentukan role EFEKTIF 2-bidang dari DB tiap request (real-time):
//
//	super_admin — email di SUPER_ADMIN_EMAILS (env-only, immutable, menang atas apa pun)
//	staff       — email terdaftar di platform_staff (support platform, mutable)
//	<membership> — role TENANT (owner/admin/member) di WORKSPACE AKTIF
//
// Role tenant kini PER-WORKSPACE (tabel memberships), bukan properti user: orang
// yang sama bisa owner di satu workspace & member di workspace lain. Platform role
// di-overlay di sini, tak pernah disimpan di DB sebagai role tenant.
//
// q HARUS bisa membaca platform_staff & memberships — keduanya TANPA RLS, jadi
// terbaca di WithTenant maupun WithSuper tx. Fail-soft: error DB → role terendah
// (member), jangan naikkan otoritas karena glitch.
func (h *Handler) resolveRole(ctx context.Context, q *db.Queries, u db.User, tenantID int64) string {
	if isSuperAdminEmail(u.Email) {
		return authz.RoleNameSuperAdmin // env override menang atas semua
	}
	if ok, err := q.IsPlatformStaff(ctx, u.Email); err != nil {
		h.Log.Error("resolveRole: staff lookup", "err", err)
	} else if ok {
		return authz.RoleNameStaff
	}
	m, err := q.GetMembership(ctx, db.GetMembershipParams{UserID: u.ID, TenantID: tenantID})
	if err != nil {
		// Tak ada membership (mis. baru dicabut) → otoritas terendah, BUKAN naik.
		h.Log.Warn("resolveRole: membership tak ditemukan", "user_id", u.ID, "tenant_id", tenantID)
		return authz.RoleNameMember
	}
	return m.Role
}
