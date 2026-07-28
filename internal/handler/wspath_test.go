package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/go-chi/chi/v5"
)

// wspath_test.go — keputusan 0004: workspace hidup di PATH. Dua hal yang dijaga
// di sini: pembentukan URL (wsPath) dan penerjemahan slug→tenant yang menolak
// workspace milik orang lain (resolveTenantBySlug).

func TestWsPath(t *testing.T) {
	cases := []struct {
		slug, sub, want string
	}{
		{"acme", "", "/w/acme"},
		{"acme", "/", "/w/acme"},
		{"acme", "/members", "/w/acme/members"},
		{"acme", "members", "/w/acme/members"}, // slash opsional — tak boleh jadi "/w/acmemembers"
		{"acme", "/settings", "/w/acme/settings"},
		// Tanpa slug (belum punya workspace) → satu-satunya tujuan yang masuk akal.
		// Yang DILARANG di sini adalah menghasilkan "/w//members" yang rusak senyap.
		{"", "/members", "/workspace/new"},
		{"", "", "/workspace/new"},
	}
	for _, c := range cases {
		if got := wsPath(c.slug, c.sub); got != c.want {
			t.Errorf("wsPath(%q, %q) = %q, want %q", c.slug, c.sub, got, c.want)
		}
	}
}

// TestSlugFromRequest: slug dibaca dari pola route chi. Route tanpa {workspace}
// (mis. /dev, /notifications) HARUS mengembalikan "" agar Scope jatuh ke session
// alih-alih menyangka ada slug.
func TestSlugFromRequest(t *testing.T) {
	r := chi.NewRouter()
	var got string
	r.Get("/w/{workspace}/members", func(w http.ResponseWriter, req *http.Request) {
		got = slugFromRequest(req)
	})
	r.Get("/notifications", func(w http.ResponseWriter, req *http.Request) {
		got = slugFromRequest(req)
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/w/acme/members", nil))
	if got != "acme" {
		t.Errorf("slug dari /w/acme/members = %q, want acme", got)
	}
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/notifications", nil))
	if got != "" {
		t.Errorf("route tanpa {workspace} harus beri \"\", got %q", got)
	}
}

// TestResolveTenantBySlug_MilikSendiri: jalur bahagia — slug milik user
// diterjemahkan ke tenant_id-nya.
func TestResolveTenantBySlug_MilikSendiri(t *testing.T) {
	env, uid := setupTest(t)
	env.withSession(t, uid, func(sc sessionCtx) {
		got, ok := env.h.resolveTenantBySlug(sc.ctx, uid, "test")
		if !ok {
			t.Fatal("slug workspace sendiri harus dikenali")
		}
		if got.ID != env.tenantID {
			t.Errorf("tenant = %d, want %d", got.ID, env.tenantID)
		}
		// Session ikut mengikuti workspace yang dibuka (session = PETUNJUK).
		if s := session.TenantSlug(sc.ctx); s != "test" {
			t.Errorf("session slug = %q, want test", s)
		}
	})
}

// TestResolveTenantBySlug_WorkspaceOrangLain: INTI KEAMANAN 0004. Slug yang
// ADA tapi bukan milik user harus ditolak — kalau lolos, mengetik slug orang
// lain di address bar = membaca data workspace mereka.
func TestResolveTenantBySlug_WorkspaceOrangLain(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()

	asing, err := env.q.CreateTenant(ctx, db.CreateTenantParams{Name: "Asing", Slug: "asing"})
	if err != nil {
		t.Fatalf("seed tenant asing: %v", err)
	}
	env.seedMember(t, "orang2@local", "owner", asing.ID)

	env.withSession(t, uid, func(sc sessionCtx) {
		if _, ok := env.h.resolveTenantBySlug(sc.ctx, uid, "asing"); ok {
			t.Fatal("KEBOCORAN: slug workspace orang lain diterima")
		}
		// Session TIDAK boleh ikut berpindah ke workspace asing.
		if s := session.TenantSlug(sc.ctx); s == "asing" {
			t.Error("session tak boleh pindah ke workspace yang ditolak")
		}
	})
}

// TestResolveTenantBySlug_SlugTakAda: slug tak dikenal ditolak dengan cara yang
// SAMA PERSIS dengan slug milik orang lain. Membedakan keduanya (mis. 404 vs
// 403) akan membocorkan workspace mana yang ada.
func TestResolveTenantBySlug_SlugTakAda(t *testing.T) {
	env, uid := setupTest(t)
	env.withSession(t, uid, func(sc sessionCtx) {
		if _, ok := env.h.resolveTenantBySlug(sc.ctx, uid, "tidak-ada-sama-sekali"); ok {
			t.Error("slug tak dikenal harus ditolak")
		}
	})
}

// TestAdoptTenantBySlug_PlatformIkutPath: REGRESI. Role platform bypass RLS dan
// TIDAK butuh membership — tapi slug di URL tetap harus diikuti. Tanpa ini,
// super_admin membuka /w/asing/members melihat anggota workspace yang kebetulan
// aktif di session-nya, di bawah URL yang menjanjikan "asing": salah data,
// senyap, dan justru di panel yang paling berwenang.
func TestAdoptTenantBySlug_PlatformIkutPath(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()

	asing, err := env.q.CreateTenant(ctx, db.CreateTenantParams{Name: "Asing", Slug: "asing"})
	if err != nil {
		t.Fatalf("seed tenant asing: %v", err)
	}

	env.withSession(t, uid, func(sc sessionCtx) {
		// Mulai dari workspace seed, lalu "buka" workspace lain lewat slug.
		session.SetActiveTenant(sc.ctx, env.tenantID, "Test", "test")
		if !env.h.adoptTenantBySlug(sc.ctx, "asing") {
			t.Fatal("platform harus bisa membuka workspace mana pun lewat slug")
		}
		if got := session.TenantID(sc.ctx); got != asing.ID {
			t.Errorf("konteks aktif = %d, want %d (harus ikut PATH, bukan session)", got, asing.ID)
		}
		// Slug tak dikenal tetap ditolak — bypass RLS bukan berarti bypass 404.
		if env.h.adoptTenantBySlug(sc.ctx, "tak-ada") {
			t.Error("slug tak dikenal harus ditolak walau role platform")
		}
	})
}

// TestHomeFor_PlatformVsTenant: role platform berumah di /dev (lintas-workspace,
// tak punya slug); role tenant berumah di ruang kerja aktif. Ini yang dulu
// dikerjakan authz.HomePath dan sengaja dipindah karena butuh slug.
func TestHomeFor_PlatformVsTenant(t *testing.T) {
	if got := homeFor(ctxWithRole(t, "super_admin")); got != "/dev" {
		t.Errorf("super_admin home = %q, want /dev", got)
	}
	if got := homeFor(ctxWithRole(t, "staff")); got != "/dev" {
		t.Errorf("staff home = %q, want /dev", got)
	}
	for _, role := range []string{"owner", "admin", "member"} {
		if got := homeFor(ctxWithRole(t, role)); got != "/w/acme" {
			t.Errorf("role %s home = %q, want /w/acme", role, got)
		}
	}
}
