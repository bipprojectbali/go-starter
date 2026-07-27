package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// runRefresh menjalankan RefreshIdentity dengan session yang HANYA berisi userID
// (mensimulasikan session lama sebelum role ditambahkan). Mengembalikan recorder
// + role yang terbaca setelah refresh.
func (e *testEnv) runRefresh(t *testing.T, userID int64, setupSuper func(string) bool) (*httptest.ResponseRecorder, string) {
	t.Helper()
	SetSuperAdminChecker(setupSuper)
	t.Cleanup(func() { SetSuperAdminChecker(func(string) bool { return false }) })

	var seenRole string
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenRole = session.Role(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	rec := httptest.NewRecorder()
	wrapped := e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.SetUserID(r.Context(), userID) // HANYA userID — role kosong (session lama)
		// Inject Queries ber-scope (RefreshIdentity pakai h.q) — sim. Scope middleware.
		r = r.WithContext(withQueries(r.Context(), e.q))
		e.h.RefreshIdentity(final).ServeHTTP(w, r)
	}))
	wrapped.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/user", nil))
	return rec, seenRole
}

func TestRefreshIdentity_SelfHealMissingRole(t *testing.T) {
	env, uid := setupTest(t)
	// Seed test@local role = "admin" di DB (role tenant valid).
	if err := env.q.UpdateUserRole(t.Context(), db.UpdateUserRoleParams{ID: uid, Role: "admin"}); err != nil {
		t.Fatalf("set role: %v", err)
	}
	rec, role := env.runRefresh(t, uid, func(string) bool { return false })
	if rec.Code != http.StatusOK {
		t.Fatalf("harus lolos, got %d", rec.Code)
	}
	// Session lama tanpa role → terisi ulang dari DB (role tenant, bukan platform).
	if role != "admin" {
		t.Errorf("role harus di-heal jadi admin (dari DB), got %q", role)
	}
}

func TestRefreshIdentity_RootEnvOverride(t *testing.T) {
	env, uid := setupTest(t)
	// DB role = user, tapi email = super-admin env → efektif super_admin.
	rec, role := env.runRefresh(t, uid, func(email string) bool { return email == "test@local" })
	if rec.Code != http.StatusOK {
		t.Fatalf("root harus lolos, got %d", rec.Code)
	}
	if role != "super_admin" {
		t.Errorf("root env harus jadi super_admin walau kolom DB user, got %q", role)
	}
}

func TestRefreshIdentity_BlockedForcesLogout(t *testing.T) {
	env, uid := setupTest(t)
	if err := env.q.UpdateUserStatus(t.Context(), db.UpdateUserStatusParams{ID: uid, Status: "blocked"}); err != nil {
		t.Fatalf("block: %v", err)
	}
	rec, _ := env.runRefresh(t, uid, func(string) bool { return false })
	// Blocked → redirect ke login (logout paksa real-time).
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("user blocked harus di-redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login?err=inactive" {
		t.Errorf("harus ke /login?err=inactive, got %q", loc)
	}
}

func TestRefreshIdentity_RootBypassesBlock(t *testing.T) {
	env, uid := setupTest(t)
	// Root env di-set blocked di DB → tetap lolos (kebal gate status).
	if err := env.q.UpdateUserStatus(t.Context(), db.UpdateUserStatusParams{ID: uid, Status: "blocked"}); err != nil {
		t.Fatalf("block: %v", err)
	}
	rec, role := env.runRefresh(t, uid, func(email string) bool { return email == "test@local" })
	if rec.Code != http.StatusOK {
		t.Fatalf("root env kebal block, harus lolos, got %d", rec.Code)
	}
	if role != "super_admin" {
		t.Errorf("root tetap super_admin, got %q", role)
	}
}
