// Package mw berisi custom middleware.
package mw

import (
	"net/http"

	"go_starter/internal/session"

	"github.com/starfederation/datastar-go/datastar"
)

// RequireAuth menolak request tanpa session user. Sadar Datastar: untuk request
// Datastar (header Datastar-Request), redirect via event SSE, bukan HTTP 303.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if session.UserID(r.Context()) == 0 {
			if r.Header.Get("Datastar-Request") == "true" {
				sse := datastar.NewSSE(w, r)
				_ = sse.Redirect("/login")
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
