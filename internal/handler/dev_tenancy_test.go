package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/appmode"
	"go_starter/internal/session"
)

// dev_tenancy_test.go — kenaikan mode dari panel platform (0007).
//
// Yang dijaga: aksi PERMANEN tak boleh bisa dipicu tanpa konfirmasi yang
// disengaja, dan kenaikannya harus benar-benar berlaku (bukan cuma menulis baris
// yang baru terbaca setelah restart).

// postTenancy menjalankan DevSettingsTenancy dengan nilai konfirmasi tertentu.
func postTenancy(t *testing.T, env *testEnv, uid int64, confirm string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"confirm": {confirm}}
	req := httptest.NewRequest(http.MethodPost, "/dev/settings/tenancy",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	env.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.SetIdentity(r.Context(), uid, "root@local", "super_admin", true,
			env.tenantID, "Test", "test", "")
		env.h.DevSettingsTenancy(w, r)
	})).ServeHTTP(rec, req)
	return rec
}

// TestDevTenancy_TolakKonfirmasiSalah: gerbang utama. Aksi ini tak bisa
// dibatalkan, jadi memicunya tanpa sengaja harus MUSTAHIL — bukan sekadar sulit.
// Yang paling mungkin terjadi di lapangan: form dikirim kosong (autofill,
// double-submit, atau tombol ditekan tanpa membaca).
func TestDevTenancy_TolakKonfirmasiSalah(t *testing.T) {
	env, uid := setupTest(t)
	prim := seedPrimary(t, env)

	withMode(t, appmode.Single, func() {
		for _, salah := range []string{"", "   ", "workspace lain", prim.Name + "x"} {
			rec := postTenancy(t, env, uid, salah)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=confirm") {
				t.Errorf("konfirmasi %q harus ditolak, got Location=%q", salah, loc)
			}
			if appmode.IsMulti() {
				t.Fatalf("konfirmasi %q TIDAK BOLEH menaikkan mode", salah)
			}
		}
	})
}

// TestDevTenancy_TerimaKonfirmasiBenar: sisi lain gerbang, plus bukti bahwa
// kenaikan berlaku SEKETIKA — kalau hanya baris DB yang tertulis, instance yang
// melayani tetap berperilaku single sampai restart, persis yang dihindari 0007.
//
// Perbandingan sengaja case-insensitive & di-trim: yang diuji adalah PERHATIAN
// operator, bukan ketelitiannya mengetik.
func TestDevTenancy_TerimaKonfirmasiBenar(t *testing.T) {
	env, uid := setupTest(t)
	prim := seedPrimary(t, env)

	withMode(t, appmode.Single, func() {
		rec := postTenancy(t, env, uid, "  "+strings.ToUpper(prim.Name)+"  ")
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=tenancy") {
			t.Fatalf("konfirmasi benar harus lolos, got Location=%q", loc)
		}
		if !appmode.IsMulti() {
			t.Error("kenaikan harus berlaku SEKETIKA pada proses yang melayani")
		}
		s, err := env.q.GetSetting(t.Context(), appmode.SettingKey)
		if err != nil {
			t.Fatalf("baca setting: %v", err)
		}
		if s.Value != appmode.NameMulti {
			t.Errorf("baris DB harus %q, got %q", appmode.NameMulti, s.Value)
		}
	})
}

// TestDevTenancy_TerekamAudit: perubahan bentuk aplikasi yang tak bisa
// dibatalkan WAJIB meninggalkan jejak "siapa & kapan". Tanpa ini, pertanyaan
// pertama saat ada yang bingung ("kenapa aplikasi ini jadi multi?") tak punya
// jawaban.
func TestDevTenancy_TerekamAudit(t *testing.T) {
	env, uid := setupTest(t)
	prim := seedPrimary(t, env)

	withMode(t, appmode.Single, func() {
		postTenancy(t, env, uid, prim.Name)
	})

	logs, err := env.q.ListAuditLogs(t.Context(), 20)
	if err != nil {
		t.Fatalf("baca audit: %v", err)
	}
	var found bool
	for _, l := range logs {
		if l.Action == "platform.tenancy.upgrade" {
			found = true
		}
	}
	if !found {
		t.Error("kenaikan mode harus tercatat di audit_logs")
	}
}

// TestDevTenancy_SudahMultiBukanError: operator menekan tombol lama di tab yang
// belum disegarkan. Itu bukan pelanggaran — memarahinya dengan halaman error tak
// membantu siapa pun, dan keadaan akhirnya memang sudah sesuai keinginannya.
func TestDevTenancy_SudahMultiBukanError(t *testing.T) {
	env, uid := setupTest(t)
	seedPrimary(t, env)

	withMode(t, appmode.Multi, func() {
		rec := postTenancy(t, env, uid, "apa saja")
		loc := rec.Header().Get("Location")
		if strings.Contains(loc, "err=") {
			t.Errorf("sudah multi tak boleh jadi error, got Location=%q", loc)
		}
	})
}

// TestDevTenancy_TanpaWorkspacePrimerDitolak: keadaan yang seharusnya mustahil
// (workspace primer dibuat saat boot), tapi kalau terjadi, kenaikan harus GAGAL
// — bukan lolos dengan konfirmasi kosong yang kebetulan cocok dengan nama kosong.
func TestDevTenancy_TanpaWorkspacePrimerDitolak(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()
	// Pastikan tak ada yang primer.
	if _, err := env.h.Pool.Exec(ctx, "UPDATE tenants SET is_primary = false"); err != nil {
		t.Fatalf("bersihkan primer: %v", err)
	}

	withMode(t, appmode.Single, func() {
		rec := postTenancy(t, env, uid, "")
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=") {
			t.Errorf("tanpa workspace primer harus gagal, got Location=%q", loc)
		}
		if appmode.IsMulti() {
			t.Error("TIDAK BOLEH naik tanpa workspace primer")
		}
	})
}
