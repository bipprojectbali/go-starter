package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go_starter/internal/appmode"
	"go_starter/internal/session"
)

// guest_test.go — gerbang halaman-tamu (login/register).
//
// Yang dijaga di sini bukan kerapian, melainkan dua hal yang sungguh merugikan:
// halaman yang berbicara dua hal bertentangan (header menampilkan email yang
// SEDANG masuk, isinya menawarkan "Masuk"), dan form yang bila diisi MENGGANTI
// sesi aktif tanpa pernah menyebutkannya.

// doGuest menjalankan RequireGuest dengan/tanpa sesi, lalu melaporkan status,
// tujuan pengalihan, dan apakah handler di belakangnya sempat dipanggil.
func doGuest(t *testing.T, env *testEnv, uid int64, role string) (int, string, bool) {
	t.Helper()
	var lolos bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lolos = true
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	env.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if uid != 0 {
			session.SetIdentity(r.Context(), uid, "orang@local", role, role == "super_admin",
				env.tenantID, "Test", "test", "")
		}
		env.h.RequireGuest(inner).ServeHTTP(w, r)
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))

	return rec.Code, rec.Header().Get("Location"), lolos
}

// TestRequireGuest_TamuLolos: sisi dasar. Tanpa ini, "selalu redirect" juga akan
// lulus test di bawah — dan halaman login jadi tak bisa dibuka siapa pun.
func TestRequireGuest_TamuLolos(t *testing.T) {
	env, _ := setupTest(t)

	code, _, lolos := doGuest(t, env, 0, "")
	if !lolos {
		t.Error("pengunjung tanpa sesi harus bisa membuka halaman login")
	}
	if code != http.StatusOK {
		t.Errorf("tamu harus lolos, got %d", code)
	}
}

// TestRequireGuest_SudahMasukDiantarKeRumah: INTI perbaikan ini. Sebelumnya
// halaman login terbuka bagi yang sudah login — dan mengisi formnya memanggil
// startIdentity → session.Renew, yaitu MENGGANTI sesi aktif secara senyap.
//
// Diantar (303), bukan ditolak: membuka /login saat sudah masuk hampir selalu
// berarti "bawa saya ke aplikasi" (tautan lama, bookmark, tombol back).
func TestRequireGuest_SudahMasukDiantarKeRumah(t *testing.T) {
	env, uid := setupTest(t)

	withMode(t, appmode.Multi, func() {
		code, loc, lolos := doGuest(t, env, uid, "owner")
		if lolos {
			t.Error("yang sudah masuk TAK BOLEH melihat halaman login")
		}
		if code != http.StatusSeeOther {
			t.Errorf("harus 303 (pola auth, bukan SSE — gotcha #16), got %d", code)
		}
		if loc == "" || loc == "/login" {
			t.Errorf("harus diantar keluar dari /login, got Location=%q", loc)
		}
	})
}

// TestRequireGuest_TujuanIkutRole: alasan gerbang ini ada di paket handler dan
// bukan di mw — tujuannya bergantung ROLE, dan itu pengetahuan handler (mw
// sengaja tak tahu bentuk URL workspace).
//
// Kalau tujuannya di-hardcode "/", platform akan mendarat di landing lalu harus
// mengklik lagi; kalau di-hardcode "/dev", anggota biasa mendarat di 403.
func TestRequireGuest_TujuanIkutRole(t *testing.T) {
	env, uid := setupTest(t)

	withMode(t, appmode.Multi, func() {
		_, locPlatform, _ := doGuest(t, env, uid, "super_admin")
		if locPlatform != "/dev" {
			t.Errorf("platform harus diantar ke /dev, got %q", locPlatform)
		}

		_, locTenant, _ := doGuest(t, env, uid, "owner")
		if locTenant != "/w/test" {
			t.Errorf("anggota tenant harus diantar ke workspace aktifnya, got %q", locTenant)
		}
	})
}
