package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go_starter/internal/appmode"
	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/go-chi/chi/v5"
)

// appmode_test.go — keputusan 0006 & 0007. Yang dijaga di sini adalah hal yang
// paling mudah rusak diam-diam: perilaku yang hanya diuji di SATU mode, lalu mode
// yang lain patah tanpa ada yang tahu. Karena itu tiap kasus di bawah dijalankan
// di kedua mode, bukan di mode yang kebetulan aktif.

// withMode menjalankan fn dalam mode tertentu dan SELALU memulihkannya.
// Mode adalah state paket (di-set sekali saat startup di produksi), jadi test
// yang lupa memulihkan akan meracuni test berikutnya — dan gejalanya muncul di
// tempat yang tak ada hubungannya.
func withMode(t *testing.T, m appmode.Mode, fn func()) {
	t.Helper()
	prev := appmode.Current()
	appmode.Set(m)
	t.Cleanup(func() { appmode.Set(prev) })
	fn()
}

// TestWsPath_SatuBentukDuaMode: INTI 0007. Mode single dulu memakai bentuk URL
// sendiri (/app/...), sehingga menaikkan aplikasi ke multi mengubah SETIAP alamat
// yang sudah tersebar — bookmark, tautan di email, dokumentasi turunan. Dengan
// satu bentuk, kenaikan mode tak menyentuh satu tautan pun.
func TestWsPath_SatuBentukDuaMode(t *testing.T) {
	cases := map[string]string{
		"":          "/w/app",
		"/":         "/w/app",
		"/members":  "/w/app/members",
		"/settings": "/w/app/settings",
	}
	// Hasilnya HARUS identik di kedua mode. Kalau kelak seseorang menambahkan
	// cabang mode di wsPath, salah satu iterasi ini gagal.
	for _, m := range []appmode.Mode{appmode.Single, appmode.Multi} {
		withMode(t, m, func() {
			for sub, want := range cases {
				if got := wsPath(appmode.PrimarySlug, sub); got != want {
					t.Errorf("%s: wsPath(app, %q) = %q, want %q", m, sub, got, want)
				}
			}
			// Slug lain pun berbentuk sama — tak ada yang istimewa soal "app".
			if got := wsPath("acme", "/members"); got != "/w/acme/members" {
				t.Errorf("%s: wsPath(acme, /members) = %q", m, got)
			}
			// Tanpa slug = belum punya workspace.
			if got := wsPath("", "/members"); got != "/workspace/new" {
				t.Errorf("%s: tanpa slug harus ke /workspace/new, got %q", m, got)
			}
		})
	}
}

// TestSlugFromRequest_SelaluDariPath: sejak 0007 tak ada lagi cabang mode di
// sini. Sebelumnya fungsi ini harus MENGARANG slug di mode single (route /app tak
// punya segmen untuk dibaca) — satu tempat yang harus tahu sedang di mode apa,
// dan satu asumsi yang meleset begitu mode bisa berubah saat jalan.
func TestSlugFromRequest_SelaluDariPath(t *testing.T) {
	for _, m := range []appmode.Mode{appmode.Single, appmode.Multi} {
		withMode(t, m, func() {
			var got string
			r := chi.NewRouter()
			r.Get("/w/{workspace}/members", func(w http.ResponseWriter, req *http.Request) {
				got = slugFromRequest(req)
			})
			r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/w/app/members", nil))
			if got != appmode.PrimarySlug {
				t.Errorf("%s: harus membaca slug dari path, got %q", m, got)
			}
		})
	}
}

// TestAssignableRoles_SingleTanpaOwner: mode single tak mengenal `owner`
// (0006 §7). Kalau dropdown masih menawarkannya, seseorang bisa diangkat jadi
// pemilik aplikasi yang seharusnya milik operator.
func TestAssignableRoles_SingleTanpaOwner(t *testing.T) {
	single := authz.AssignableRoles(true)
	for _, r := range single {
		if r == authz.RoleNameOwner {
			t.Fatal("mode single TAK BOLEH menawarkan role owner")
		}
	}
	if len(single) != 2 {
		t.Errorf("single harus menawarkan member+admin saja, got %v", single)
	}
	// Multi tetap lengkap — owner adalah pemilik workspace yang sah di sana.
	multi := authz.AssignableRoles(false)
	if len(multi) != 3 {
		t.Errorf("multi harus menawarkan member+admin+owner, got %v", multi)
	}
}

// TestCanEditWorkspace_AdminHanyaDiSingle: admin adalah PEMBANTU super_admin —
// ia mengurus operasional (termasuk nama aplikasi) di mode single, tempat tak
// ada owner. Di mode multi owner-lah yang berwenang, jadi admin tetap hanya
// melihat; melonggarkannya di sana akan diam-diam memperluas wewenang admin di
// setiap workspace orang lain.
func TestCanEditWorkspace_AdminHanyaDiSingle(t *testing.T) {
	withMode(t, appmode.Multi, func() {
		if canEditWorkspace(ctxWithRole(t, authz.RoleNameAdmin)) {
			t.Error("multi: admin TAK boleh mengubah pengaturan workspace")
		}
		if !canEditWorkspace(ctxWithRole(t, authz.RoleNameOwner)) {
			t.Error("multi: owner harus boleh")
		}
	})

	withMode(t, appmode.Single, func() {
		if !canEditWorkspace(ctxWithRole(t, authz.RoleNameAdmin)) {
			t.Error("single: admin harus boleh mengatur aplikasi (0006 §7)")
		}
		if canEditWorkspace(ctxWithRole(t, authz.RoleNameMember)) {
			t.Error("single: member tetap tak boleh")
		}
	})
}

// TestBootstrapPrimary_MembuatWorkspacePrimer: aplikasi tak pernah berada di
// keadaan "belum ada workspace" — keadaan paling jarang diuji adalah yang paling
// sering rusak.
func TestBootstrapPrimary_MembuatWorkspacePrimer(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	// setupTest menyeed satu tenant biasa; buang agar DB benar-benar kosong —
	// meniru deployment baru.
	if _, err := env.h.Pool.Exec(ctx, "TRUNCATE tenants RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("kosongkan tenants: %v", err)
	}

	mode, err := BootstrapPrimary(ctx, env.h.Pool, "Aplikasi Saya")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	// DB kosong = SINGLE, tanpa siapa pun perlu mengisi env apa pun.
	if mode != appmode.Single {
		t.Errorf("deployment baru harus lahir sebagai single, got %s", mode)
	}
	tn, err := env.q.GetPrimaryTenant(ctx)
	if err != nil {
		t.Fatalf("workspace primer harus ada setelah bootstrap: %v", err)
	}
	if tn.Name != "Aplikasi Saya" {
		t.Errorf("nama dari APP_NAME, got %q", tn.Name)
	}
	if tn.Slug != appmode.PrimarySlug {
		t.Errorf("slug primer harus %q, got %q", appmode.PrimarySlug, tn.Slug)
	}

	// Idempoten: boot kedua tak boleh membuat duplikat. Kalau ia mencoba, unique
	// partial index di tenants yang akan menolaknya — dan error itu muncul di
	// sini, bukan sebagai dua "rumah aplikasi" yang diam-diam hidup bersama.
	if _, err := BootstrapPrimary(ctx, env.h.Pool, "Aplikasi Saya"); err != nil {
		t.Fatalf("bootstrap kedua: %v", err)
	}
	var n int64
	if err := env.h.Pool.QueryRow(ctx, "SELECT count(*) FROM tenants WHERE is_primary").Scan(&n); err != nil {
		t.Fatalf("hitung primer: %v", err)
	}
	if n != 1 {
		t.Errorf("boot berulang harus tetap 1 workspace primer, got %d", n)
	}
}

// TestBootstrapPrimary_MembacaModeDariDB: mode datang dari DATABASE, bukan env
// (0007). Env bisa dibalik; baris DB dijaga trigger yang menolak penurunan.
func TestBootstrapPrimary_MembacaModeDariDB(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()

	// Belum ada baris → single.
	mode, err := BootstrapPrimary(ctx, env.h.Pool, "App")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if mode != appmode.Single {
		t.Errorf("tanpa baris tenancy_mode harus single, got %s", mode)
	}

	// Dinaikkan → multi terbaca di boot berikutnya.
	if _, err := env.h.Pool.Exec(ctx,
		"INSERT INTO platform_settings(key,value) VALUES ($1,$2)",
		appmode.SettingKey, appmode.NameMulti); err != nil {
		t.Fatalf("seed mode: %v", err)
	}
	mode, err = BootstrapPrimary(ctx, env.h.Pool, "App")
	if err != nil {
		t.Fatalf("bootstrap kedua: %v", err)
	}
	if mode != appmode.Multi {
		t.Errorf("harus membaca multi dari DB, got %s", mode)
	}
}

// TestBootstrapPrimary_TolakNilaiTakDikenal: nilai TERISI tapi ngawur = data
// rusak, bukan keadaan awal. Diam-diam jatuh ke single berarti menyembunyikan
// setiap workspace selain yang primer — kehilangan data yang tampak seperti bug UI.
func TestBootstrapPrimary_TolakNilaiTakDikenal(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	if _, err := env.h.Pool.Exec(ctx,
		"INSERT INTO platform_settings(key,value) VALUES ($1,$2)",
		appmode.SettingKey, "sesuatu-yang-lain"); err != nil {
		t.Fatalf("seed mode: %v", err)
	}
	if _, err := BootstrapPrimary(ctx, env.h.Pool, "App"); err == nil {
		t.Fatal("nilai tenancy_mode tak dikenal HARUS menggagalkan boot")
	}
}

// TestRatchetMode_MultiTakBisaTurun: INTI 0007, dan dijaga DATABASE — bukan kode
// yang ingat memeriksa. Keempat jalur diuji karena tiga di antaranya pernah
// terlewat saat dirancang: UPDATE langsung, UPSERT (jalur yang dipakai aplikasi),
// dan DELETE (baris yang absen dibaca sebagai single, jadi menghapusnya adalah
// penurunan yang menyamar).
func TestRatchetMode_MultiTakBisaTurun(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()

	naik := func() {
		t.Helper()
		if _, err := env.h.Pool.Exec(ctx,
			`INSERT INTO platform_settings(key,value) VALUES ($1,$2)
			 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
			appmode.SettingKey, appmode.NameMulti); err != nil {
			t.Fatalf("naik ke multi harus BOLEH: %v", err)
		}
	}
	naik()

	turun := []struct {
		nama string
		sql  string
	}{
		{"UPDATE langsung", `UPDATE platform_settings SET value='single' WHERE key='` + appmode.SettingKey + `'`},
		{"UPSERT", `INSERT INTO platform_settings(key,value) VALUES ('` + appmode.SettingKey + `','single')
		            ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`},
		{"DELETE (ketiadaan = single)", `DELETE FROM platform_settings WHERE key='` + appmode.SettingKey + `'`},
	}
	for _, c := range turun {
		if _, err := env.h.Pool.Exec(ctx, c.sql); err == nil {
			t.Errorf("%s: penurunan mode HARUS ditolak database", c.nama)
		}
	}

	// Naik lagi (idempoten) tetap boleh — ratchet menahan arah, bukan gerakan.
	naik()

	// Key LAIN tak boleh ikut terkunci: triggernya khusus tenancy_mode.
	if _, err := env.h.Pool.Exec(ctx,
		"INSERT INTO platform_settings(key,value) VALUES ('workspace_quota_default','5')"); err != nil {
		t.Fatalf("seed key lain: %v", err)
	}
	if _, err := env.h.Pool.Exec(ctx,
		"DELETE FROM platform_settings WHERE key='workspace_quota_default'"); err != nil {
		t.Errorf("key lain harus tetap bebas diubah/dihapus: %v", err)
	}
}

// TestPlaceNewUser_SingleGabungSebagaiMember: celah yang paling mudah terlewat
// (0006 §6). Tanpa ini, SETIAP orang yang mendaftar jadi owner aplikasi.
func TestPlaceNewUser_SingleGabungSebagaiMember(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	// Siapkan workspace primer.
	if _, err := env.q.CreatePrimaryTenant(ctx, db.CreatePrimaryTenantParams{
		Name: "App", Slug: appmode.PrimarySlug,
	}); err != nil {
		t.Fatalf("seed workspace primer: %v", err)
	}
	sebelum := env.countTenants(t)

	u := env.seedUserOnly(t, "pendaftar@local")

	withMode(t, appmode.Single, func() {
		tn, err := placeNewUser(ctx, env.q, u.ID, "Nama Yang Diminta")
		if err != nil {
			t.Fatalf("placeNewUser: %v", err)
		}
		if tn.Slug != appmode.PrimarySlug {
			t.Errorf("harus gabung ke aplikasi tunggal, got %q", tn.Slug)
		}
		// TIDAK membuat workspace baru.
		sesudah := env.countTenants(t)
		if sesudah != sebelum {
			t.Errorf("mode single TAK BOLEH membuat workspace baru: %d → %d", sebelum, sesudah)
		}
		// Dan BUKAN owner.
		m, err := env.q.GetMembership(ctx, db.GetMembershipParams{UserID: u.ID, TenantID: tn.ID})
		if err != nil {
			t.Fatalf("membership: %v", err)
		}
		if m.Role != authz.RoleNameMember {
			t.Errorf("pendaftar di mode single harus MEMBER, got %q — setiap orang jadi owner aplikasi", m.Role)
		}
	})
}

// TestPlaceNewUser_MultiBuatWorkspaceOwner: perilaku mode multi tak boleh ikut
// berubah — di sana pendaftar memang pemilik workspace-nya sendiri.
func TestPlaceNewUser_MultiBuatWorkspaceOwner(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	u := env.seedUserOnly(t, "pemilik@local")

	withMode(t, appmode.Multi, func() {
		tn, err := placeNewUser(ctx, env.q, u.ID, "Punya Saya")
		if err != nil {
			t.Fatalf("placeNewUser: %v", err)
		}
		if tn.Name != "Punya Saya" {
			t.Errorf("workspace baru harus bernama sesuai permintaan, got %q", tn.Name)
		}
		m, err := env.q.GetMembership(ctx, db.GetMembershipParams{UserID: u.ID, TenantID: tn.ID})
		if err != nil {
			t.Fatalf("membership: %v", err)
		}
		if m.Role != authz.RoleNameOwner {
			t.Errorf("pendaftar di mode multi harus OWNER workspace-nya, got %q", m.Role)
		}
	})
}

// TestWorkspaceOptions_SingleTanpaSwitcher: switcher & tombol buat-baru tak
// punya arti saat hanya ada satu aplikasi.
func TestWorkspaceOptions_SingleTanpaSwitcher(t *testing.T) {
	env, uid := setupTest(t)
	withMode(t, appmode.Single, func() {
		env.withSession(t, uid, func(sc sessionCtx) {
			session.SetActiveTenant(sc.ctx, env.tenantID, "Test", "test")
			ws, canCreate := env.h.workspaceOptions(sc.ctx)
			if len(ws) != 0 || canCreate {
				t.Errorf("single: switcher harus kosong & tak bisa buat baru, got %d/%v", len(ws), canCreate)
			}
		})
	})
}

// TestUpgradeToMulti_BerlakuSeketikaDanSekaliJalan: jalur kenaikan yang dipakai
// operator. Tiga hal sekaligus, karena ketiganya harus benar BERSAMA agar
// kenaikan tanpa restart itu sah:
//
//  1. baris DB tertulis (sumber kebenaran, dibaca boot berikutnya);
//  2. cache settings & state proses ikut berubah (kalau tidak, instance yang
//     melayani tetap berperilaku single sampai restart — persis yang dihindari);
//  3. tak ada jalan kembali (tak ada DowngradeToMulti, dan DB menolaknya).
func TestUpgradeToMulti_BerlakuSeketikaDanSekaliJalan(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	u := env.seedUserOnly(t, "operator@local")

	// Mulai dari single supaya perubahannya benar-benar teruji.
	withMode(t, appmode.Single, func() {
		if err := UpgradeToMulti(ctx, env.q, u.ID); err != nil {
			t.Fatalf("UpgradeToMulti: %v", err)
		}
		// (2) state proses — inilah yang membuat "tanpa restart" berarti.
		if !appmode.IsMulti() {
			t.Error("mode proses harus SEKETIKA jadi multi (tanpa restart)")
		}
		// (1) baris DB.
		s, err := env.q.GetSetting(ctx, appmode.SettingKey)
		if err != nil {
			t.Fatalf("baca setting: %v", err)
		}
		if s.Value != appmode.NameMulti {
			t.Errorf("nilai DB harus %q, got %q", appmode.NameMulti, s.Value)
		}
		// Idempoten: operator menekan dua kali tak boleh jadi error.
		if err := UpgradeToMulti(ctx, env.q, u.ID); err != nil {
			t.Errorf("kenaikan berulang harus aman: %v", err)
		}
		// (3) tak ada jalan kembali — dijaga DB, bukan oleh ketiadaan fungsi.
		if _, err := env.h.Pool.Exec(ctx,
			"UPDATE platform_settings SET value='single' WHERE key=$1", appmode.SettingKey); err == nil {
			t.Error("penurunan HARUS ditolak database")
		}
		// Boot berikutnya membaca multi.
		mode, err := BootstrapPrimary(ctx, env.h.Pool, "App")
		if err != nil {
			t.Fatalf("bootstrap setelah upgrade: %v", err)
		}
		if mode != appmode.Multi {
			t.Errorf("boot setelah upgrade harus multi, got %s", mode)
		}
	})
}
