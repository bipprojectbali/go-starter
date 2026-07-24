package handler

import (
	"context"
	"net/http"
	"strings"

	"go_stater/internal/authz"
	"go_stater/internal/db"
	"go_stater/internal/session"
	"go_stater/internal/ui"
	"go_stater/internal/ui/pages/panel"

	"github.com/starfederation/datastar-go/datastar"
	g "maragu.dev/gomponents"
)

// workspaceName adalah bentuk signal form ganti nama workspace.
type workspaceName struct {
	Name string `json:"name"`
}

// canEditWorkspace melaporkan apakah aktor boleh mengubah nama workspace:
// owner (pemilik) atau platform (super_admin/staff yang membantu). Admin/member
// TIDAK — mereka hanya lihat. Root env selalu boleh (super_admin efektif).
func canEditWorkspace(ctx context.Context) bool {
	if session.IsRoot(ctx) {
		return true
	}
	role := session.Role(ctx)
	return role == authz.RoleNameOwner || isPlatformRole(role)
}

// WorkspaceSettings — GET /admin/workspace. Form nama workspace (AppShell).
// Semua penghuni /admin boleh LIHAT; hanya owner/platform boleh ubah (canEdit).
func (h *Handler) WorkspaceSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t, err := h.q(ctx).GetTenant(ctx, session.TenantID(ctx))
	if err != nil {
		h.Log.Error("workspace: get tenant", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.renderShell(w, r, "Workspace", "go_stater /admin", "/admin/workspace", adminNav,
		panel.Workspace(t.Name, t.Slug, canEditWorkspace(ctx)))
}

// WorkspaceUpdate — POST /admin/workspace. Ganti nama workspace (owner/platform).
// Guard di handler (bukan cuma route) karena admin BOLEH akses /admin tapi TAK
// boleh ganti nama. Slug tak diubah (immutable). Audit + refresh brand sidebar.
func (h *Handler) WorkspaceUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canEditWorkspace(ctx) {
		h.workspaceMsg(w, r, "Hanya owner yang dapat mengubah nama workspace")
		return
	}
	var in workspaceName
	if err := datastar.ReadSignals(r, &in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		h.workspaceMsg(w, r, "Nama workspace wajib diisi")
		return
	}
	if len(name) > maxWorkspaceNameLen {
		h.workspaceMsg(w, r, "Nama workspace maksimal 60 karakter")
		return
	}

	tenantID := session.TenantID(ctx)
	if err := h.q(ctx).UpdateTenant(ctx, db.UpdateTenantParams{ID: tenantID, Name: name}); err != nil {
		h.Log.Error("workspace: update", "err", err)
		h.workspaceMsg(w, r, "Gagal menyimpan perubahan")
		return
	}
	// Segarkan brand sidebar seketika (tanpa tunggu request berikutnya).
	session.SetTenantName(ctx, name)
	// Audit (fail-soft). target = tenant sendiri; metadata TANPA nama (bukan PII,
	// tapi konsisten: id saja). actor = user aktif.
	h.audit(ctx, session.UserID(ctx), "workspace.rename", tenantID, nil)
	h.workspaceOK(w, r, "Nama workspace disimpan")
}

// workspaceMsg mem-patch alert error (id "workspace-msg") via Datastar SSE.
func (h *Handler) workspaceMsg(w http.ResponseWriter, r *http.Request, msg string) {
	patch(w, r, h.Log, ui.Alert(ui.VariantDestructive, "workspace-msg", g.Text(msg)))
}

// workspaceOK mem-patch alert sukses (id "workspace-msg") via Datastar SSE.
func (h *Handler) workspaceOK(w http.ResponseWriter, r *http.Request, msg string) {
	patch(w, r, h.Log, ui.Alert(ui.VariantDefault, "workspace-msg", g.Text(msg)))
}
