package handler

import (
	"net/http"

	"go_starter/internal/session"
)

// guest.go — gerbang untuk halaman yang HANYA masuk akal bagi pengunjung yang
// belum masuk: login & register.
//
// Kebalikan dari mw.RequireAuth, dan sama pentingnya. Tanpa ini halaman login
// tetap terbuka bagi yang sudah login — bukan cuma janggal, tapi menyesatkan:
// header di atasnya menampilkan email orang yang SEDANG masuk, sementara isinya
// menawarkan "Masuk" dan "Belum punya akun? Daftar". Satu halaman berbicara dua
// hal yang bertentangan, dan yang paling mungkin dilakukan orang adalah mengira
// sesinya hilang.
//
// Lebih buruk lagi: mengisi form itu MENGGANTI sesi yang sedang aktif tanpa
// pernah menyebut bahwa itu yang terjadi (startIdentity memanggil session.Renew).
//
// Kenapa di paket handler, bukan mw: tujuan pengalihannya bergantung ROLE
// (`homeFor` — platform ke /dev, tenant ke workspace aktifnya), dan itu
// pengetahuan handler. mw sengaja tak tahu apa-apa soal bentuk URL workspace.
func (h *Handler) RequireGuest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if session.UserID(r.Context()) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		// Sudah masuk → antar ke rumahnya, bukan tolak dengan error. Membuka
		// /login saat sudah masuk hampir selalu berarti "bawa saya ke aplikasi"
		// (tautan lama, bookmark, tombol back), jadi memarahinya tak membantu.
		//
		// 303 (bukan 302): pola yang sama dipakai seluruh jalur auth, sebab
		// redirect lewat SSE diblokir CSP (gotcha #16).
		http.Redirect(w, r, homeFor(r.Context()), http.StatusSeeOther)
	})
}
