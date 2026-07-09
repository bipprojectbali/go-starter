package mw

import (
	"net/http"

	"go_stater/internal/authz"

	"github.com/starfederation/datastar-go/datastar"
)

// RequireEnforce menolak request bila user login tak punya izin (obj, act) per
// Casbin. Pasang SETELAH RequireAuth (authn dulu, baru authz). Datastar-aware:
// untuk request Datastar balas flash SSE, selainnya HTTP 403.
func RequireEnforce(obj, act string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authz.Can(r.Context(), obj, act) {
				next.ServeHTTP(w, r)
				return
			}
			if r.Header.Get("Datastar-Request") == "true" {
				sse := datastar.NewSSE(w, r)
				_ = sse.PatchElements(
					`<div id="flash" class="alert alert-error" role="alert">Akses ditolak</div>`)
				return
			}
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	}
}
