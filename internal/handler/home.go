package handler

import (
	"net/http"

	"go_stater/internal/authz"
	"go_stater/internal/session"
	"go_stater/internal/ui/pages"
)

// Home — GET / (publik). User yang SUDAH login diarahkan ke home per-role
// (mis. super_admin → /dev); anonim melihat landing page.
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	if session.UserID(r.Context()) != 0 {
		http.Redirect(w, r, authz.HomePathFor(session.Role(r.Context())), http.StatusSeeOther)
		return
	}
	h.renderPage(w, r, "go_stater", pages.Landing(false))
}
