package handler

import (
	"net/http"
	"strings"
)

// urls.go — URL ABSOLUT aplikasi (yang keluar dari proses: redirect OAuth, tautan
// undangan). Berbeda dari wspath.go yang mengurus path RELATIF di dalam app.
//
// Kenapa dipusatkan: sebelumnya path callback OAuth ditulis DUA KALI — di
// routes.go sebagai route, dan di env GOOGLE_REDIRECT_URL sebagai teks. Dua
// salinan satu kebenaran; salah ketik di salah satunya berujung
// `redirect_uri_mismatch` dari Google, gejala yang tak menyebut sebabnya.
// Sekarang env hanya menyimpan BASE URL, path dirakit dari konstanta yang sama
// dengan route-nya.

// PathGoogleCallback = path callback OAuth Google. Dipakai routes.go (mendaftar
// route) DAN redirect_uri yang dikirim ke Google — satu konstanta, jadi keduanya
// mustahil berbeda.
//
// Prefix /api/auth/ dipertahankan agar exact-match dengan Authorized redirect
// URIs yang sudah terdaftar di Google Cloud Console.
const PathGoogleCallback = "/api/auth/callback/google"

// appBaseURL = alamat publik aplikasi tanpa trailing slash (mis.
// "https://app.example.com"), di-inject saat startup — pola SetCSSPath/SetDevMode.
var appBaseURL string

// SetAppBaseURL menetapkan alamat publik aplikasi. Nilai kosong berarti "tak
// diketahui": pemakainya lalu jatuh ke penebakan dari request (lihat baseFrom).
func SetAppBaseURL(u string) { appBaseURL = strings.TrimRight(u, "/") }

// GoogleRedirectURL merakit redirect_uri lengkap untuk dikirim ke Google.
// Kosong bila base URL belum di-set — pemanggil (main) memperlakukannya seperti
// kredensial Google yang tak lengkap: OAuth tak diaktifkan.
func GoogleRedirectURL() string {
	if appBaseURL == "" {
		return ""
	}
	return appBaseURL + PathGoogleCallback
}

// baseFrom mengembalikan alamat publik aplikasi: APP_BASE_URL bila di-set,
// selainnya ditebak dari request.
//
// Penebakan dipertahankan HANYA sebagai jaring dev (di sana host memang
// berpindah-pindah: localhost, IP LAN, tunnel). Ia tak boleh diandalkan di
// produksi — r.Host & X-Forwarded-Proto datang dari KLIEN, jadi tautan undangan
// yang dirakit darinya bisa diarahkan ke host penyerang oleh siapa pun yang
// memicu pembuatannya.
func baseFrom(r *http.Request) string {
	if appBaseURL != "" {
		return appBaseURL
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
