package handler

import (
	"net/http"

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
		u, err := h.DB.GetUser(ctx, uid)
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

		role := u.Role
		if isRoot {
			role = "super_admin" // env override apa pun nilai kolom DB
		}
		avatar := ""
		if u.AvatarUrl != nil {
			avatar = *u.AvatarUrl
		}
		// Tulis session hanya bila ada yang berubah (hindari commit tiap request).
		if session.Role(ctx) != role || session.IsRoot(ctx) != isRoot ||
			session.Email(ctx) != u.Email || session.AvatarURL(ctx) != avatar {
			session.SetIdentity(ctx, u.ID, u.Email, role, isRoot, avatar)
		}
		next.ServeHTTP(w, r)
	})
}
