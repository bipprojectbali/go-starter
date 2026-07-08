package handler

import (
	"net/http"

	"go_stater/internal/session"
	"go_stater/internal/ui/pages"
)

// Home — GET / (publik). Landing page; TIDAK redirect ke /login. CTA
// menyesuaikan status login (dibaca dari session).
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	loggedIn := session.UserID(r.Context()) != 0
	h.renderPage(w, r, "go_stater", pages.Landing(loggedIn))
}
