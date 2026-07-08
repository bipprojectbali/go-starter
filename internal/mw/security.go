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
		// CSP: script/style dari origin sendiri (Datastar & CSS vendored, bukan CDN).
		//
		// 'unsafe-eval' WAJIB untuk Datastar: engine ekspresinya mengevaluasi
		// atribut data-* lewat new Function() (lihat data-star.dev/reference/security).
		// Tanpa ini SELURUH Datastar mati — @post/signals/patch tak jalan. Ini
		// direkomendasikan docs resmi Datastar; permukaan serang tak bertambah
		// signifikan karena Datastar memang mengevaluasi ekspresi.
		//
		// 'unsafe-inline' style: Datastar & komponen men-inject style kecil.
		// font-src: izinkan font data-uri (Basecoat) + self.
		// img-src: googleusercontent untuk avatar Google (lh3–lh6).
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-eval'; "+
				"style-src 'self' 'unsafe-inline'; "+
				"font-src 'self' data:; "+
				"img-src 'self' data: https://*.googleusercontent.com")
		next.ServeHTTP(w, r)
	})
}
