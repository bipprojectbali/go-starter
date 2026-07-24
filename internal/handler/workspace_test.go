package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go_stater/internal/session"
)

// doWorkspace menjalankan handler workspace dgn session ber-role tertentu +
// Queries ber-scope (sim. Scope middleware). body = JSON signal (utk POST).
func (e *testEnv) doWorkspace(uid int64, role, body string, fn http.HandlerFunc) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/admin/workspace", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	wrapped := e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// tenantID = env.tenantID (workspace seed); nama awal "Test".
		session.SetIdentity(r.Context(), uid, "test@local", role, false, e.tenantID, "Test", "")
		fn(w, r.WithContext(withQueries(r.Context(), e.q)))
	}))
	wrapped.ServeHTTP(rec, req)
	return rec
}

// TestWorkspaceUpdate_OwnerRenames: owner ganti nama → tersimpan + audit + flash.
func TestWorkspaceUpdate_OwnerRenames(t *testing.T) {
	env, uid := setupTest(t)
	rec := env.doWorkspace(uid, "owner", `{"name":"Acme Baru"}`, env.h.WorkspaceUpdate)

	tn, err := env.q.GetTenant(t.Context(), env.tenantID)
	if err != nil {
		t.Fatalf("get tenant: %v", err)
	}
	if tn.Name != "Acme Baru" {
		t.Errorf("nama harus berubah jadi 'Acme Baru', got %q", tn.Name)
	}
	// Slug TAK berubah (immutable) — tetap "test" (nilai seed setupTest).
	if tn.Slug != "test" {
		t.Errorf("slug harus tetap (immutable), got %q", tn.Slug)
	}
	if !strings.Contains(rec.Body.String(), "disimpan") {
		t.Errorf("flash sukses harus ada:\n%s", rec.Body.String())
	}
	// Audit tercatat.
	logs, _ := env.q.ListAuditLogs(t.Context(), 10)
	if len(logs) == 0 {
		t.Error("rename harus tercatat di audit_logs")
	}
}

// TestWorkspaceUpdate_AdminDenied: admin (bukan owner) → ditolak, nama tak berubah.
func TestWorkspaceUpdate_AdminDenied(t *testing.T) {
	env, uid := setupTest(t)
	rec := env.doWorkspace(uid, "admin", `{"name":"Curang"}`, env.h.WorkspaceUpdate)

	tn, _ := env.q.GetTenant(t.Context(), env.tenantID)
	if tn.Name == "Curang" {
		t.Error("admin tak boleh ganti nama workspace")
	}
	if !strings.Contains(rec.Body.String(), "owner") {
		t.Errorf("flash harus tolak (owner-only):\n%s", rec.Body.String())
	}
}

// TestWorkspaceUpdate_EmptyName: nama kosong ditolak (owner sekalipun).
func TestWorkspaceUpdate_EmptyName(t *testing.T) {
	env, uid := setupTest(t)
	rec := env.doWorkspace(uid, "owner", `{"name":"   "}`, env.h.WorkspaceUpdate)

	if !strings.Contains(rec.Body.String(), "wajib diisi") {
		t.Errorf("nama kosong harus ditolak:\n%s", rec.Body.String())
	}
}
