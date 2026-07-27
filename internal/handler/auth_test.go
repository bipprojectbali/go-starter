package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/auth"
	"go_starter/internal/db"
)

// postForm membangun request POST form-encoded (native form, bukan JSON/Datastar).
func postForm(path string, form url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestRegister_Success(t *testing.T) {
	env, _ := setupTest(t)

	rec := env.do(postForm("/register", url.Values{
		"workspace": {"Acme Corp"}, "email": {"baru@local"}, "password": {"rahasia123"},
	}), env.h.Register)

	// Native PRG: sukses → 303 + Location home (owner tenant baru → /admin).
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("want 303, got %d: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/admin" {
		t.Errorf("register sukses (owner) harus redirect /admin, got %q", loc)
	}
	// REGRESI: cookie WAJIB terkirim (native redirect → scs LoadAndSave commit).
	if c := rec.Header().Get("Set-Cookie"); !strings.Contains(c, "session=") {
		t.Errorf("Set-Cookie session harus ada, got %q", c)
	}
	// User tersimpan dengan hash argon2id (bukan plaintext).
	u, err := env.q.GetUserByEmail(t.Context(), "baru@local")
	if err != nil {
		t.Fatalf("user tidak tersimpan: %v", err)
	}
	if u.PassHash == nil || !strings.HasPrefix(*u.PassHash, "$argon2id$") {
		t.Errorf("password tidak di-hash argon2id: %v", u.PassHash)
	}
	// Workspace baru terbuat dgn NAMA input user (bukan email) + slug ter-slugify.
	tn, err := env.q.GetTenant(t.Context(), u.TenantID)
	if err != nil {
		t.Fatalf("tenant tak tersimpan: %v", err)
	}
	if tn.Name != "Acme Corp" {
		t.Errorf("nama workspace = input user, got %q", tn.Name)
	}
	if tn.Slug != "acme-corp" {
		t.Errorf("slug ter-slugify dari nama, got %q", tn.Slug)
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	env, _ := setupTest(t)
	rec := env.do(postForm("/register", url.Values{
		"workspace": {"WS"}, "email": {"x@local"}, "password": {"pendek"},
	}), env.h.Register)

	// Gagal → 303 ke /register?err=short (pola PRG).
	if loc := rec.Header().Get("Location"); loc != "/register?err=short" {
		t.Errorf("password <8 harus redirect ?err=short, got %q", loc)
	}
}

func TestRegister_MissingWorkspace(t *testing.T) {
	env, _ := setupTest(t)
	rec := env.do(postForm("/register", url.Values{
		"email": {"x@local"}, "password": {"rahasia123"},
	}), env.h.Register)

	if loc := rec.Header().Get("Location"); loc != "/register?err=workspace" {
		t.Errorf("nama workspace kosong harus redirect ?err=workspace, got %q", loc)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	env, _ := setupTest(t)
	// user seed "test@local" sudah ada.
	rec := env.do(postForm("/register", url.Values{
		"workspace": {"Dup"}, "email": {"test@local"}, "password": {"rahasia123"},
	}), env.h.Register)

	if loc := rec.Header().Get("Location"); loc != "/register?err=exists" {
		t.Errorf("email duplikat harus redirect ?err=exists, got %q", loc)
	}
}

func TestLogin_Success(t *testing.T) {
	env, _ := setupTest(t)
	hash, _ := auth.HashPassword("rahasia123")
	if _, err := env.q.CreateUser(t.Context(), db.CreateUserParams{Email: "login@local", PassHash: &hash, TenantID: env.tenantID, Role: "member"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := env.do(postForm("/login", url.Values{
		"email": {"login@local"}, "password": {"rahasia123"},
	}), env.h.Login)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("want 303, got %d", rec.Code)
	}
	// member → /user (home role).
	if loc := rec.Header().Get("Location"); loc != "/user" {
		t.Errorf("login sukses (member) harus redirect /user, got %q", loc)
	}
	if c := rec.Header().Get("Set-Cookie"); !strings.Contains(c, "session=") {
		t.Errorf("Set-Cookie session harus ada, got %q", c)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	env, _ := setupTest(t)
	hash, _ := auth.HashPassword("benar")
	env.q.CreateUser(t.Context(), db.CreateUserParams{Email: "u@local", PassHash: &hash, TenantID: env.tenantID, Role: "member"})

	rec := env.do(postForm("/login", url.Values{
		"email": {"u@local"}, "password": {"salah"},
	}), env.h.Login)

	// Kode generik (anti user-enumeration).
	if loc := rec.Header().Get("Location"); loc != "/login?err=invalid" {
		t.Errorf("password salah harus redirect ?err=invalid, got %q", loc)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	env, _ := setupTest(t)
	rec := env.do(postForm("/login", url.Values{
		"email": {"nobody@local"}, "password": {"apa"},
	}), env.h.Login)

	// Kode generik SAMA dengan wrong password (anti user-enumeration).
	if loc := rec.Header().Get("Location"); loc != "/login?err=invalid" {
		t.Errorf("email tak dikenal harus redirect ?err=invalid (generik), got %q", loc)
	}
}

// TestAuthErrMsg: pemetaan kode → pesan (yang dirender di halaman auth).
func TestAuthErrMsg(t *testing.T) {
	cases := map[string]string{
		"invalid":   "Email atau password salah",
		"exists":    "Email sudah terdaftar",
		"inactive":  "Akun tidak aktif. Hubungi administrator.",
		"workspace": "Nama workspace wajib diisi",
		"":          "",
		"unknown":   "",
	}
	for code, want := range cases {
		if got := authErrMsg(code); got != want {
			t.Errorf("authErrMsg(%q)=%q, want %q", code, got, want)
		}
	}
}
