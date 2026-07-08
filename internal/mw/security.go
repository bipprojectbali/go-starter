package mw

import "net/http"

// SecurityHeaders memasang header keamanan dasar pada tiap response.
// Setara "helmet" minimal — tanpa dependency.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// CSP: izinkan script/style dari origin sendiri (Datastar & CSS vendored,
		// bukan CDN). 'unsafe-inline' untuk style karena Datastar men-inject sedikit.
		// img-src izinkan googleusercontent untuk avatar Google (lh3–lh6).
		h.Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https://*.googleusercontent.com")
		next.ServeHTTP(w, r)
	})
}
