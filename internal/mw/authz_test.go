package mw

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go_stater/internal/authz"
	"go_stater/internal/session"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
)

// setupAuthz menyiapkan enforcer (policy embed) + session manager memstore.
func setupAuthz(t *testing.T) *scs.SessionManager {
	t.Helper()
	e, err := authz.New(authz.Model, authz.Policy)
	if err != nil {
		t.Fatalf("authz.New: %v", err)
	}
	authz.Init(e)

	sm := scs.New()
	sm.Store = memstore.New()
	sm.Lifetime = time.Hour
	session.Init(sm)
	return sm
}

// runWithRole menjalankan handler yang diproteksi RequireEnforce dalam session
// dengan role tertentu. Mengembalikan status code.
func runWithRole(t *testing.T, sm *scs.SessionManager, role, obj, act string) int {
	t.Helper()
	reached := false
	guarded := RequireEnforce(obj, act)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	wrapped := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if role != "" {
			session.SetIdentity(r.Context(), 1, "u@x.com", role, false, "")
		}
		guarded.ServeHTTP(w, r)
	}))
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dev/users", nil))
	if rec.Code == http.StatusOK && !reached {
		t.Fatal("status 200 tapi handler tak tercapai")
	}
	return rec.Code
}

func TestRequireEnforce(t *testing.T) {
	sm := setupAuthz(t)

	// /dev = super_admin saja.
	if code := runWithRole(t, sm, "super_admin", "dev:users", "read"); code != http.StatusOK {
		t.Errorf("super_admin harus lolos /dev, got %d", code)
	}
	if code := runWithRole(t, sm, "admin", "dev:users", "read"); code != http.StatusForbidden {
		t.Errorf("admin harus 403 di /dev, got %d", code)
	}
	if code := runWithRole(t, sm, "user", "dev:users", "read"); code != http.StatusForbidden {
		t.Errorf("user harus 403 di /dev, got %d", code)
	}
	// Anonim (tanpa role di session) → ditolak.
	if code := runWithRole(t, sm, "", "dev:users", "read"); code != http.StatusForbidden {
		t.Errorf("anonim harus 403, got %d", code)
	}
}

func TestRequireEnforce_RootBypass(t *testing.T) {
	sm := setupAuthz(t)
	// Root (isRoot=true) lolos apa pun, walau role bukan super_admin.
	reached := false
	guarded := RequireEnforce("dev:users", "read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	wrapped := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.SetIdentity(r.Context(), 1, "root@x.com", "user", true, "") // role=user tapi isRoot
		guarded.ServeHTTP(w, r)
	}))
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dev/users", nil))
	if rec.Code != http.StatusOK || !reached {
		t.Errorf("root harus bypass semua, got %d reached=%v", rec.Code, reached)
	}
}
