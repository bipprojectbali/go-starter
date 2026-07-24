package handler

import (
	"context"
	"net/http"

	"go_stater/internal/authz"
	"go_stater/internal/db"
	"go_stater/internal/session"
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

		role := h.resolveRole(ctx, h.q(ctx), u)
		avatar := ""
		if u.AvatarUrl != nil {
			avatar = *u.AvatarUrl
		}
		// Nama workspace segar dari DB (real-time: ganti nama langsung terlihat di
		// brand sidebar). tenants TANPA RLS-policy → terbaca di tx ber-scope. Fail-
		// soft: error → pakai nama tercache (jangan kosongkan brand karena glitch).
		tenantName := session.TenantName(ctx)
		if t, err := h.q(ctx).GetTenant(ctx, u.TenantID); err == nil {
			tenantName = t.Name
		}
		// Tulis session hanya bila ada yang berubah (hindari commit tiap request).
		if session.Role(ctx) != role || session.IsRoot(ctx) != isRoot ||
			session.Email(ctx) != u.Email || session.AvatarURL(ctx) != avatar ||
			session.TenantID(ctx) != u.TenantID || session.TenantName(ctx) != tenantName {
			session.SetIdentity(ctx, u.ID, u.Email, role, isRoot, u.TenantID, tenantName, avatar)
		}
		next.ServeHTTP(w, r)
	})
}

// resolveRole menentukan role EFEKTIF 2-bidang dari DB tiap request (real-time):
//
//	super_admin — email di SUPER_ADMIN_EMAILS (env-only, immutable, menang atas apa pun)
//	staff       — email terdaftar di platform_staff (support platform, mutable)
//	<u.Role>    — role tenant (owner/admin/member) dari kolom users.role
//
// Platform role di-overlay di sini, TAK disimpan di users.role (kolom itu CHECK
// owner/admin/member). q HARUS bisa membaca platform_staff — tabel itu TANPA RLS
// (platform-scope), jadi terbaca di WithTenant maupun WithSuper tx. Lookup staff
// fail-soft: error DB → fallback ke role tenant (jangan kunci user karena glitch).
func (h *Handler) resolveRole(ctx context.Context, q *db.Queries, u db.User) string {
	if isSuperAdminEmail(u.Email) {
		return authz.RoleNameSuperAdmin // env override menang atas semua
	}
	if ok, err := q.IsPlatformStaff(ctx, u.Email); err != nil {
		h.Log.Error("resolveRole: staff lookup", "err", err)
	} else if ok {
		return authz.RoleNameStaff
	}
	return u.Role
}
