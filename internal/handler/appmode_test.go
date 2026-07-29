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

// appmode_test.go — keputusan 0006. Yang dijaga di sini adalah hal yang paling
// mudah rusak diam-diam: jalur yang membentuk URL hanya diuji di SATU mode, lalu
// mode yang lain patah tanpa ada yang tahu. Karena itu tiap kasus di bawah
// dijalankan di kedua mode, bukan di mode yang kebetulan aktif.

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

// TestWsPath_BentukPerMode: satu fungsi, dua bentuk URL. Inilah alasan 0006
// murah — seluruh handler & view sudah lewat sini sejak 0004.
func TestWsPath_BentukPerMode(t *testing.T) {
	withMode(t, appmode.Multi, func() {
		cases := map[string]string{
			"":          "/w/acme",
			"/":         "/w/acme",
			"/members":  "/w/acme/members",
			"/settings": "/w/acme/settings",
		}
		for sub, want := range cases {
			if got := wsPath("acme", sub); got != want {
				t.Errorf("multi wsPath(acme, %q) = %q, want %q", sub, got, want)
			}
		}
		// Tanpa slug di mode multi = belum punya workspace.
		if got := wsPath("", "/members"); got != "/workspace/new" {
			t.Errorf("multi tanpa slug harus ke /workspace/new, got %q", got)
		}
	})

	withMode(t, appmode.Single, func() {
		cases := map[string]string{
			"":          "/app",
			"/":         "/app",
			"/members":  "/app/members",
			"/settings": "/app/settings",
		}
		for sub, want := range cases {
			// Slug diabaikan di mode single — apa pun isinya, hasilnya sama.
			for _, slug := range []string{"", "acme", appmode.SingleSlug} {
				if got := wsPath(slug, sub); got != want {
					t.Errorf("single wsPath(%q, %q) = %q, want %q", slug, sub, got, want)
				}
			}
		}
	})
}

// TestSlugFromRequest_SingleSelaluTerisi: kunci kenapa Scope tak butuh cabang
// mode. Route /app tak punya segmen slug untuk dibaca chi; kalau helper ini
// mengembalikan "", Scope akan jatuh ke jalur "tanpa slug" dan mode single
// kehilangan seluruh penegakan berbasis slug.
func TestSlugFromRequest_SingleSelaluTerisi(t *testing.T) {
	withMode(t, appmode.Single, func() {
		// Request polos, TANPA route context chi sama sekali.
		req := httptest.NewRequest(http.MethodGet, "/app/members", nil)
		if got := slugFromRequest(req); got != appmode.SingleSlug {
			t.Errorf("single harus selalu memberi slug tunggal, got %q", got)
		}
	})

	withMode(t, appmode.Multi, func() {
		var got string
		r := chi.NewRouter()
		r.Get("/w/{workspace}/members", func(w http.ResponseWriter, req *http.Request) {
			got = slugFromRequest(req)
		})
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/w/acme/members", nil))
		if got != "acme" {
			t.Errorf("multi harus membaca slug dari path, got %q", got)
		}
	})
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

// TestBootstrapSingleApp_MembuatTenantTunggal: aplikasi tak pernah berada di
// keadaan "belum ada workspace" — keadaan paling jarang diuji adalah yang paling
// sering rusak.
func TestBootstrapSingleApp_MembuatTenantTunggal(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	// setupTest menyeed satu tenant ber-slug "test"; buang agar DB benar-benar kosong.
	if _, err := env.h.Pool.Exec(ctx, "TRUNCATE tenants RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("kosongkan tenants: %v", err)
	}

	withMode(t, appmode.Single, func() {
		if err := BootstrapSingleApp(ctx, env.h.Pool, "Aplikasi Saya"); err != nil {
			t.Fatalf("bootstrap: %v", err)
		}
		tn, err := env.q.GetTenantBySlug(ctx, appmode.SingleSlug)
		if err != nil {
			t.Fatalf("tenant tunggal harus ada setelah bootstrap: %v", err)
		}
		if tn.Name != "Aplikasi Saya" {
			t.Errorf("nama dari APP_NAME, got %q", tn.Name)
		}
		// Idempoten: boot kedua tak boleh membuat duplikat.
		if err := BootstrapSingleApp(ctx, env.h.Pool, "Aplikasi Saya"); err != nil {
			t.Fatalf("bootstrap kedua: %v", err)
		}
		n, _ := env.q.CountTenants(ctx)
		if n != 1 {
			t.Errorf("boot berulang harus tetap 1 tenant, got %d", n)
		}
	})
}

// TestBootstrapSingleApp_AdopsiTenantKosong: REGRESI (ditemukan saat menjalankan
// app sungguhan di DB baru). Migrasi 00007 membuat tenant "default" sebagai wadah
// backfill, jadi SETIAP database baru punya tepat satu tenant ber-slug yang salah
// — dan mode single langsung menolak start sebelum sempat dipakai.
//
// Workspace KOSONG boleh diadopsi. Yang sudah berisi anggota tidak: mengubah
// slug-nya mematikan setiap tautan yang sudah tersebar (alasan slug immutable
// sejak 0004), jadi di situ operator yang memutuskan.
func TestBootstrapSingleApp_AdopsiTenantKosong(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	// setupTest menyeed tenant "test" + SATU anggota. Buang anggotanya agar
	// workspace jadi kosong — meniru tenant "default" bawaan migrasi.
	if _, err := env.h.Pool.Exec(ctx, "TRUNCATE memberships"); err != nil {
		t.Fatalf("kosongkan memberships: %v", err)
	}

	withMode(t, appmode.Single, func() {
		if err := BootstrapSingleApp(ctx, env.h.Pool, "Aplikasi Saya"); err != nil {
			t.Fatalf("workspace kosong harus bisa diadopsi: %v", err)
		}
		tn, err := env.q.GetTenantBySlug(ctx, appmode.SingleSlug)
		if err != nil {
			t.Fatalf("slug harus jadi %q: %v", appmode.SingleSlug, err)
		}
		if tn.Name != "Aplikasi Saya" {
			t.Errorf("nama ikut disesuaikan APP_NAME, got %q", tn.Name)
		}
		n, _ := env.q.CountTenants(ctx)
		if n != 1 {
			t.Errorf("adopsi TAK BOLEH membuat tenant baru, got %d", n)
		}
	})
}

// TestBootstrapSingleApp_TolakAdopsiTenantBerisi: batasnya. Workspace yang sudah
// dipakai orang tak boleh diubah slug-nya diam-diam — tautan tersimpan mati.
func TestBootstrapSingleApp_TolakAdopsiTenantBerisi(t *testing.T) {
	env, _ := setupTest(t) // tenant "test" DENGAN satu anggota

	withMode(t, appmode.Single, func() {
		if err := BootstrapSingleApp(t.Context(), env.h.Pool, "App"); err == nil {
			t.Fatal("workspace berisi anggota TAK BOLEH diadopsi diam-diam")
		}
	})
}

// TestBootstrapSingleApp_TolakBanyakTenant: INTI PENGAMAN 0006 §10. Memilih
// diam-diam salah satu berarti workspace lain lenyap dari pandangan tanpa jejak
// — kehilangan data yang terlihat seperti bug UI.
func TestBootstrapSingleApp_TolakBanyakTenant(t *testing.T) {
	env, _ := setupTest(t) // sudah punya 1 tenant ("test")
	ctx := t.Context()
	if _, err := env.q.CreateTenant(ctx, db.CreateTenantParams{Name: "Kedua", Slug: "kedua"}); err != nil {
		t.Fatalf("seed tenant kedua: %v", err)
	}

	withMode(t, appmode.Single, func() {
		err := BootstrapSingleApp(ctx, env.h.Pool, "App")
		if err == nil {
			t.Fatal("APP_MODE=single dgn >1 workspace HARUS menolak start")
		}
	})

	// Mode multi tak peduli berapa banyak — itu memang bentuknya.
	withMode(t, appmode.Multi, func() {
		if err := BootstrapSingleApp(ctx, env.h.Pool, "App"); err != nil {
			t.Errorf("mode multi tak boleh terpengaruh: %v", err)
		}
	})
}

// TestPlaceNewUser_SingleGabungSebagaiMember: celah yang paling mudah terlewat
// (0006 §6). Tanpa ini, SETIAP orang yang mendaftar jadi owner aplikasi.
func TestPlaceNewUser_SingleGabungSebagaiMember(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	// Siapkan tenant tunggal ber-slug "app".
	if _, err := env.q.CreateTenant(ctx, db.CreateTenantParams{
		Name: "App", Slug: appmode.SingleSlug,
	}); err != nil {
		t.Fatalf("seed tenant app: %v", err)
	}
	sebelum, _ := env.q.CountTenants(ctx)

	u := env.seedUserOnly(t, "pendaftar@local")

	withMode(t, appmode.Single, func() {
		tn, err := placeNewUser(ctx, env.q, u.ID, "Nama Yang Diminta")
		if err != nil {
			t.Fatalf("placeNewUser: %v", err)
		}
		if tn.Slug != appmode.SingleSlug {
			t.Errorf("harus gabung ke aplikasi tunggal, got %q", tn.Slug)
		}
		// TIDAK membuat workspace baru.
		sesudah, _ := env.q.CountTenants(ctx)
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
