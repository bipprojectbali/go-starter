package mw

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go_starter/internal/appmode"
)

// tenancy_test.go — keputusan 0007. Yang dijaga di sini bukan "mode single
// menolak", melainkan bahwa penolakannya dievaluasi PER-REQUEST.

// withMode menyetel mode dan SELALU memulihkannya. Mode adalah state paket; test
// yang lupa memulihkan meracuni test lain dengan gejala di tempat tak
// berhubungan.
func withMode(t *testing.T, m appmode.Mode) {
	t.Helper()
	prev := appmode.Current()
	appmode.Set(m)
	t.Cleanup(func() { appmode.Set(prev) })
}

// probe menjalankan RequireMulti dan mengembalikan status + apakah handler di
// belakangnya sempat dipanggil.
func probe(t *testing.T) (int, bool) {
	t.Helper()
	var lolos bool
	h := RequireMulti(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lolos = true
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/workspace/new", nil))
	return rec.Code, lolos
}

// TestRequireMulti_SingleMenutupJalur: bukan cuma menyembunyikan menu — handler
// di belakangnya TIDAK BOLEH terpanggil sama sekali. Menu tersembunyi + route
// hidup = pintu belakang (kaidah 0006 §4, yang tetap berlaku).
func TestRequireMulti_SingleMenutupJalur(t *testing.T) {
	withMode(t, appmode.Single)

	code, lolos := probe(t)
	if lolos {
		t.Error("handler TAK BOLEH terpanggil di mode single")
	}
	// 404, bukan 403: di mode single route ini memang TIDAK ADA dari sudut pandang
	// pemakai. 403 menyiratkan fitur yang bisa dibuka dengan role lain — padahal
	// tak ada wewenang yang kurang di sini.
	if code != http.StatusNotFound {
		t.Errorf("mode single harus 404 (bukan 403 — tak ada wewenang yang kurang), got %d", code)
	}
}

// TestRequireMulti_MultiMeneruskan: sisi lain gerbang. Tanpa ini, "selalu 404"
// juga akan lulus test di atas.
func TestRequireMulti_MultiMeneruskan(t *testing.T) {
	withMode(t, appmode.Multi)

	code, lolos := probe(t)
	if !lolos {
		t.Error("handler harus diteruskan di mode multi")
	}
	if code != http.StatusOK {
		t.Errorf("mode multi harus lolos, got %d", code)
	}
}

// TestRequireMulti_MengikutiKenaikanTanpaRestart: INTI keputusan 0007, dan alasan
// gerbang ini berupa middleware alih-alih `if appmode.IsMulti()` saat mendaftarkan
// route.
//
// Pendaftaran route terjadi SEKALI saat boot. Mode bisa NAIK saat aplikasi
// berjalan, jadi route bersyarat akan telanjur salah sesudahnya — dan gejalanya
// adalah 404 pada menu yang baru saja muncul: kegagalan yang tampak seperti bug
// UI, bukan seperti sisa keputusan wiring.
//
// Handler di sini dirakit SEKALI (meniru pendaftaran saat boot) lalu dipanggil
// dua kali dengan mode berbeda. Kalau kelak seseorang memindahkan pemeriksaan ini
// ke waktu-pendaftaran, test ini gagal.
func TestRequireMulti_MengikutiKenaikanTanpaRestart(t *testing.T) {
	withMode(t, appmode.Single)

	var lolos bool
	h := RequireMulti(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lolos = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/workspace/new", nil)

	h.ServeHTTP(httptest.NewRecorder(), req)
	if lolos {
		t.Fatal("single: harus tertutup")
	}

	// Mode naik SAAT BERJALAN — handler yang sama, tanpa dirakit ulang.
	appmode.Set(appmode.Multi)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !lolos {
		t.Error("kenaikan mode harus berlaku SEKETIKA pada handler yang sudah terdaftar")
	}
}
