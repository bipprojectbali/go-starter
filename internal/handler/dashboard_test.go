package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go_stater/internal/authz"
	"go_stater/internal/session"
)

// renderAs menjalankan handler shell dalam session dengan role tertentu.
func (e *testEnv) renderAs(role string, fn http.HandlerFunc) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	wrapped := e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.SetIdentity(r.Context(), 1, "u@x.com", role, false, 1, "")
		fn(w, r)
	}))
	wrapped.ServeHTTP(rec, req)
	return rec
}

func TestAdminHome_Renders(t *testing.T) {
	env, _ := setupTest(t)
	rec := env.renderAs("admin", env.h.AdminHome)
	if rec.Code != http.StatusOK {
		t.Fatalf("AdminHome harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Dashboard Admin", "go_stater /admin", "aria-current"} {
		if !strings.Contains(body, want) {
			t.Errorf("admin shell kurang %q", want)
		}
	}
}

func TestUserHome_Renders(t *testing.T) {
	env, _ := setupTest(t)
	rec := env.renderAs("member", env.h.UserHome)
	if rec.Code != http.StatusOK {
		t.Fatalf("UserHome harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	// Menu user punya Beranda (/user).
	for _, want := range []string{"Beranda", "/user"} {
		if !strings.Contains(body, want) {
			t.Errorf("user shell kurang %q", want)
		}
	}
}

// TestPolicy_PanelAccess memverifikasi hierarki akses panel via enforcer nyata.
func TestPolicy_PanelAccess(t *testing.T) {
	e, err := authz.New(authz.Model, authz.Policy)
	if err != nil {
		t.Fatalf("authz.New: %v", err)
	}
	cases := []struct {
		role, obj string
		want      bool
	}{
		{"member", "user:home", true},
		{"member", "admin:home", false},
		{"user", "dev:users", false},
		{"admin", "user:home", true}, // admin mewarisi user
		{"admin", "admin:home", true},
		{"admin", "dev:users", false},
		{"super_admin", "admin:home", true},
		{"super_admin", "dev:users", true},
	}
	for _, c := range cases {
		got, err := e.Enforce(c.role, c.obj, "read")
		if err != nil {
			t.Fatalf("Enforce: %v", err)
		}
		if got != c.want {
			t.Errorf("Enforce(%s,%s,read)=%v, want %v", c.role, c.obj, got, c.want)
		}
	}
}
