package handler

import (
	"net/http"

	"go_starter/internal/session"
	"go_starter/internal/ui/pages"
)

// Home — GET / (publik). Landing page untuk SEMUA (login maupun anonim). Tidak
// redirect: user login tetap boleh melihat halaman depan. CTA menyesuaikan
// status login (login → "Buka aplikasi" ke home per-role, anonim → "Masuk").
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	loggedIn := session.UserID(r.Context()) != 0
	home := homeFor(r.Context())
	// Landing = halaman publik utama → metadata SEO lengkap (indexable). Deskripsi
	// kosong → memakai deskripsi aplikasi default (config APP_DESCRIPTION).
	h.renderPublicPage(w, r, appName, "", pages.Landing(loggedIn, home, appName))
}
