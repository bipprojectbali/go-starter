package handler

import (
	"net/http"

	"go_stater/internal/authz"
	"go_stater/internal/session"
	"go_stater/internal/ui/pages"
)

// Home — GET / (publik). Landing page untuk SEMUA (login maupun anonim). Tidak
// redirect: user login tetap boleh melihat halaman depan. CTA menyesuaikan
// status login (login → "Buka aplikasi" ke home per-role, anonim → "Masuk").
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	loggedIn := session.UserID(r.Context()) != 0
	home := authz.HomePathFor(session.Role(r.Context()))
	h.renderPage(w, r, "go_stater", pages.Landing(loggedIn, home))
}
