package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go_stater/internal/auth"
	"go_stater/internal/db"
)

func TestRegister_Success(t *testing.T) {
	env, _ := setupTest(t)

	req := httptest.NewRequest(http.MethodPost, "/register",
		strings.NewReader(`{"workspace":"Acme Corp","email":"baru@local","password":"rahasia123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := env.do(req, env.h.Register)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// Sukses → redirect ke home per-role. User baru = OWNER tenant baru → /admin.
	if !strings.Contains(rec.Body.String(), "/admin") {
		t.Errorf("register sukses (owner tenant baru) harus redirect ke /admin:\n%s", rec.Body.String())
	}
	// REGRESI: cookie WAJIB terkirim meski response via SSE (bug scs+NewSSE).
	// Tanpa assert ini, bug "session tak login" lolos test.
	if c := rec.Header().Get("Set-Cookie"); !strings.Contains(c, "session=") {
		t.Errorf("Set-Cookie session harus ada di response register, got %q", c)
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
	req := httptest.NewRequest(http.MethodPost, "/register",
		strings.NewReader(`{"workspace":"WS","email":"x@local","password":"pendek"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := env.do(req, env.h.Register)

	if !strings.Contains(rec.Body.String(), "minimal 8") {
		t.Errorf("harus tolak password <8 karakter:\n%s", rec.Body.String())
	}
}

func TestRegister_MissingWorkspace(t *testing.T) {
	env, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodPost, "/register",
		strings.NewReader(`{"email":"x@local","password":"rahasia123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := env.do(req, env.h.Register)

	if !strings.Contains(rec.Body.String(), "workspace wajib") {
		t.Errorf("harus tolak nama workspace kosong:\n%s", rec.Body.String())
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	env, _ := setupTest(t)
	// user seed "test@local" sudah ada.
	req := httptest.NewRequest(http.MethodPost, "/register",
		strings.NewReader(`{"workspace":"Dup","email":"test@local","password":"rahasia123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := env.do(req, env.h.Register)

	if !strings.Contains(rec.Body.String(), "sudah terdaftar") {
		t.Errorf("harus tolak email duplikat:\n%s", rec.Body.String())
	}
}

func TestLogin_Success(t *testing.T) {
	env, _ := setupTest(t)
	// Buat user dengan password ter-hash.
	hash, _ := auth.HashPassword("rahasia123")
	if _, err := env.q.CreateUser(t.Context(), db.CreateUserParams{Email: "login@local", PassHash: &hash, TenantID: env.tenantID, Role: "member"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"email":"login@local","password":"rahasia123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := env.do(req, env.h.Login)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "/user") {
		t.Errorf("login sukses harus redirect ke /user (home role):\n%s", rec.Body.String())
	}
	if c := rec.Header().Get("Set-Cookie"); !strings.Contains(c, "session=") {
		t.Errorf("Set-Cookie session harus ada di response login, got %q", c)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	env, _ := setupTest(t)
	hash, _ := auth.HashPassword("benar")
	env.q.CreateUser(t.Context(), db.CreateUserParams{Email: "u@local", PassHash: &hash, TenantID: env.tenantID, Role: "member"})

	req := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"email":"u@local","password":"salah"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := env.do(req, env.h.Login)

	if !strings.Contains(rec.Body.String(), "salah") {
		t.Errorf("password salah harus ditolak:\n%s", rec.Body.String())
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	env, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"email":"nobody@local","password":"apa"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := env.do(req, env.h.Login)

	// Pesan generik (anti user-enumeration) — sama dengan wrong password.
	if !strings.Contains(rec.Body.String(), "Email atau password salah") {
		t.Errorf("email tak dikenal harus pesan generik:\n%s", rec.Body.String())
	}
}
