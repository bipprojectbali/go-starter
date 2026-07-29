package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// urls_test.go — URL absolut aplikasi. Yang dijaga: path callback tak pernah
// berbeda dari route-nya, dan tautan yang keluar dari aplikasi tak dirakit dari
// masukan klien saat alamat publik sudah diketahui.

// withBaseURL menetapkan base URL dan SELALU memulihkannya. appBaseURL adalah
// state paket (di-set sekali saat startup di produksi); test yang lupa
// memulihkan akan meracuni test lain dengan gejala di tempat tak berhubungan.
func withBaseURL(t *testing.T, u string) {
	t.Helper()
	prev := appBaseURL
	SetAppBaseURL(u)
	t.Cleanup(func() { appBaseURL = prev })
}

// TestGoogleRedirectURL_MemakaiPathRoute: INTI perubahan ini. Path callback dulu
// ditulis dua kali — di routes.go dan di env GOOGLE_REDIRECT_URL — sehingga
// salah ketik di salah satunya hanya muncul sebagai `redirect_uri_mismatch`
// dari Google, pesan yang tak menyebut sebabnya.
func TestGoogleRedirectURL_MemakaiPathRoute(t *testing.T) {
	withBaseURL(t, "https://app.example.com")

	want := "https://app.example.com" + PathGoogleCallback
	if got := GoogleRedirectURL(); got != want {
		t.Errorf("GoogleRedirectURL() = %q, want %q", got, want)
	}
	// Path-nya harus persis yang terdaftar di Google Console milik user
	// (25 URI, semuanya berakhiran ini). Mengubahnya = semua deployment mati.
	if PathGoogleCallback != "/api/auth/callback/google" {
		t.Errorf("path callback berubah (%q) — akan memutus SEMUA redirect URI "+
			"yang sudah terdaftar di Google Cloud Console", PathGoogleCallback)
	}
}

// TestSetAppBaseURL_MembuangTrailingSlash: "https://x.com/" + "/api/..." akan
// menghasilkan "//api/..." yang TIDAK cocok dengan Google Console — dan trailing
// slash adalah hal yang paling mudah ikut ter-paste.
func TestSetAppBaseURL_MembuangTrailingSlash(t *testing.T) {
	withBaseURL(t, "https://app.example.com/")

	want := "https://app.example.com" + PathGoogleCallback
	if got := GoogleRedirectURL(); got != want {
		t.Errorf("trailing slash harus dibuang: got %q, want %q", got, want)
	}
}

// TestGoogleRedirectURL_KosongTanpaBase: tanpa alamat publik, redirect_uri tak
// bisa dirakit. Mengembalikan "" (bukan path telanjang) agar pemanggil
// memperlakukannya seperti kredensial tak lengkap — OAuth tak diaktifkan,
// alih-alih mengirim redirect_uri cacat yang ditolak Google.
func TestGoogleRedirectURL_KosongTanpaBase(t *testing.T) {
	withBaseURL(t, "")
	if got := GoogleRedirectURL(); got != "" {
		t.Errorf("tanpa base URL harus kosong, got %q", got)
	}
}

// TestBaseFrom_BaseURLMenangAtasHeader: r.Host & X-Forwarded-Proto datang dari
// KLIEN. Bila alamat publik sudah diketahui, ia harus menang — kalau tidak,
// siapa pun yang memicu pembuatan undangan bisa mengarahkan tautannya ke host
// penyerang lewat header Host palsu.
func TestBaseFrom_BaseURLMenangAtasHeader(t *testing.T) {
	withBaseURL(t, "https://app.example.com")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "penyerang.example.net"
	req.Header.Set("X-Forwarded-Proto", "https")

	if got := baseFrom(req); got != "https://app.example.com" {
		t.Errorf("APP_BASE_URL harus menang atas header klien, got %q", got)
	}
}

// TestBaseFrom_MenebakSaatBelumDiSet: jaring dev — di sana host memang
// berpindah-pindah (localhost, IP LAN, tunnel), dan memaksa env hanya untuk
// `make dev` adalah gesekan tanpa manfaat.
func TestBaseFrom_MenebakSaatBelumDiSet(t *testing.T) {
	withBaseURL(t, "")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "localhost:8080"
	if got := baseFrom(req); got != "http://localhost:8080" {
		t.Errorf("dev harus menebak dari request, got %q", got)
	}

	// Di belakang proxy TLS, skema ikut header — tetap hanya untuk dev.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Host = "dev.tunnel.test"
	req2.Header.Set("X-Forwarded-Proto", "https")
	if got := baseFrom(req2); got != "https://dev.tunnel.test" {
		t.Errorf("X-Forwarded-Proto harus dihormati saat menebak, got %q", got)
	}
}

// TestInviteLink_MemakaiBaseURL: tautan undangan dikirim ke ORANG LAIN, jadi ia
// harus menunjuk alamat publik yang benar — bukan host yang kebetulan dipakai
// pembuatnya (mis. IP internal di belakang load balancer).
func TestInviteLink_MemakaiBaseURL(t *testing.T) {
	withBaseURL(t, "https://app.example.com")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "10.0.0.5:8080" // host internal, tak berguna bagi penerima

	want := "https://app.example.com/invite/tok123"
	if got := inviteLink(req, "tok123"); got != want {
		t.Errorf("inviteLink = %q, want %q", got, want)
	}
}
