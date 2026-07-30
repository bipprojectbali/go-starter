package mw

import (
	"net/http"

	"go_starter/internal/appmode"
)

// RequireMulti menutup route yang hanya masuk akal di mode multi-tenant —
// berpindah workspace dan membuat workspace baru.
//
// Kenapa middleware, bukan `if appmode.IsMulti()` saat mendaftarkan route:
// pendaftaran terjadi SEKALI saat boot, sementara mode bisa NAIK saat aplikasi
// berjalan (keputusan 0007). Route bersyarat akan telanjur salah sesudahnya, dan
// gejalanya adalah 404 pada menu yang baru saja muncul — kegagalan yang tampak
// seperti bug UI, bukan seperti sisa keputusan wiring.
//
// Tetap memenuhi kaidah 0006 §4: jalurnya benar-benar TERTUTUP di mode single,
// bukan cuma menunya disembunyikan (menu tersembunyi + route hidup = pintu
// belakang). Bedanya, penutupan itu kini dievaluasi per-request.
//
// 404 (bukan 403) karena di mode single route ini memang TIDAK ADA dari sudut
// pandang pemakai — tak ada wewenang yang kurang, dan 403 justru menyiratkan
// fitur yang bisa dibuka dengan role lain.
func RequireMulti(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if appmode.IsSingle() {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
