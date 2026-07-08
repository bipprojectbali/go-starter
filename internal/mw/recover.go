package mw

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover menangkap panic di handler, mencatat stack terstruktur, dan membalas
// 500 — koneksi tidak mati senyap. Pengganti chi middleware.Recoverer agar
// log memakai slog + request ID.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					// http.ErrAbortHandler adalah sinyal sengaja, bukan bug — teruskan.
					if v == http.ErrAbortHandler {
						panic(v)
					}
					reqLog(r, log).Error("panic recovered",
						"err", v,
						"method", r.Method,
						"path", r.URL.Path,
						"stack", string(debug.Stack()),
					)
					http.Error(w, "internal error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
