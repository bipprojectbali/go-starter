package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// setupDevUsers menyiapkan env + enforcer + super-admin checker, kembalikan
// env, id super-admin (aktor), dan id user biasa (target).
func setupDevUsers(t *testing.T) (*testEnv, int64, int64) {
	t.Helper()
	env, superID := setupTest(t) // seed test@local (role owner di DB)
	// super_admin = ENV-ONLY (checker), TAK ditulis ke users.role (CHECK hanya
	// owner/admin/member). RefreshIdentity/checker yang meng-overlay super_admin.
	SetSuperAdminChecker(func(email string) bool { return email == "test@local" })
	t.Cleanup(func() { SetSuperAdminChecker(func(string) bool { return false }) })

	// Enforcer nyata dari policy embed.
	e, err := authz.New(authz.Model, authz.Policy)
	if err != nil {
		t.Fatalf("authz.New: %v", err)
	}
	authz.Init(e)

	// User biasa (target).
	target, err := env.q.CreateUser(t.Context(), db.CreateUserParams{Email: "target@local", PassHash: ptr("x"), TenantID: env.tenantID, Role: "member"})
	if err != nil {
		t.Fatalf("seed target: %v", err)
	}
	return env, superID, target.ID
}

// doDevAction menjalankan handler dev dalam session super-admin (root) dengan
// chi URL param {id} + form values. Session di-set via LoadAndSave; identitas
// disuntikkan lalu handler dipanggil dengan request yang membawa chi param.
func (e *testEnv) doDevAction(actorID, targetID int64, form url.Values, fn http.HandlerFunc) *httptest.ResponseRecorder {
	base := httptest.NewRequest(http.MethodPost, "/dev/users/x", strings.NewReader(form.Encode()))
	base.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	base = withChiParam(base, "id", itoa(targetID))

	rec := httptest.NewRecorder()
	// scs LoadAndSave menurunkan context dari request masuk, jadi r sudah bawa
	// chi param (dari base) + session sekaligus.
	wrapped := e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.SetIdentity(r.Context(), actorID, "test@local", "super_admin", true, 1, "Acme", "")
		fn(w, r.WithContext(withQueries(r.Context(), e.q))) // inject Queries (sim. Scope)
	}))
	wrapped.ServeHTTP(rec, base)
	return rec
}

func TestDevUserSetRole_Success(t *testing.T) {
	env, superID, targetID := setupDevUsers(t)
	// Aksi kini balas SSE (200), bukan redirect. Sukses diverifikasi lewat efek
	// DB + isi flash, bukan status code.
	rec := env.doDevAction(superID, targetID, url.Values{"role": {"admin"}}, env.h.DevUserSetRole)
	u, _ := env.q.GetUser(t.Context(), targetID)
	if u.Role != "admin" {
		t.Errorf("role harus admin, got %q", u.Role)
	}
	if !strings.Contains(rec.Body.String(), "Role diubah ke admin") {
		t.Errorf("flash sukses harus ada:\n%s", rec.Body.String())
	}
	// Audit tercatat.
	logs, _ := env.q.ListAuditLogs(t.Context(), 10)
	if len(logs) == 0 {
		t.Error("aksi harus tercatat di audit_logs")
	}
}

func TestDevUserSetRole_ProtectRootEnv(t *testing.T) {
	env, superID, _ := setupDevUsers(t)
	// Target = super-admin itu sendiri (root env) → tak bisa diturunkan. Kirim role
	// VALID (member) agar lolos validasi & benar-benar menguji guard root-protect
	// (bukan tertahan "role tidak valid"). users.role tetap "owner" (nilai seed).
	rec := env.doDevAction(superID, superID, url.Values{"role": {"member"}}, env.h.DevUserSetRole)
	u, _ := env.q.GetUser(t.Context(), superID)
	if u.Role != "owner" {
		t.Errorf("role root tak boleh berubah dari nilai DB seed (owner), got %q", u.Role)
	}
	// Flash menolak (root env).
	if !strings.Contains(rec.Body.String(), "root") {
		t.Errorf("flash harus tolak root env:\n%s", rec.Body.String())
	}
}

func TestDevUserSetStatus_Block(t *testing.T) {
	env, superID, targetID := setupDevUsers(t)
	rec := env.doDevAction(superID, targetID, url.Values{"status": {"blocked"}}, env.h.DevUserSetStatus)
	u, _ := env.q.GetUser(t.Context(), targetID)
	if u.Status != "blocked" {
		t.Errorf("status harus blocked, got %q", u.Status)
	}
	if !strings.Contains(rec.Body.String(), "Status diubah ke blocked") {
		t.Errorf("flash sukses status harus ada:\n%s", rec.Body.String())
	}
}

func TestDevUserDelete_Success(t *testing.T) {
	env, superID, targetID := setupDevUsers(t)
	rec := env.doDevAction(superID, targetID, url.Values{}, env.h.DevUserDelete)
	// Soft-delete: GetUser (filter deleted_at) tak menemukannya.
	if _, err := env.q.GetUser(t.Context(), targetID); err == nil {
		t.Error("user terhapus tak boleh ditemukan GetUser")
	}
	// SSE menghapus baris + flash.
	if !strings.Contains(rec.Body.String(), "user-"+itoa(targetID)) {
		t.Errorf("SSE harus menargetkan baris user:\n%s", rec.Body.String())
	}
}

func TestDevUserDelete_SelfLockout(t *testing.T) {
	env, superID, _ := setupDevUsers(t)
	rec := env.doDevAction(superID, superID, url.Values{}, env.h.DevUserDelete)
	// Root/self → tak terhapus + flash menolak.
	if _, err := env.q.GetUser(t.Context(), superID); err != nil {
		t.Error("root/self tak boleh terhapus")
	}
	if !strings.Contains(rec.Body.String(), "root") {
		t.Errorf("flash harus tolak (root/self):\n%s", rec.Body.String())
	}
}
