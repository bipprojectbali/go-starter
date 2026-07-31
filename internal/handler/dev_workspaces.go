package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/dev"
)

// dev_workspaces.go — panel PLATFORM atas workspace: daftar lintas-workspace +
// suspend/unsuspend/restore (keputusan 0005). Terpisah dari
// workspace_lifecycle.go (aksi owner) mengikuti pemisahan kewenangannya:
// suspend adalah tindakan platform TERHADAP workspace, dan owner tak boleh
// membatalkannya sendiri — kalau bisa, gunanya hilang.

// devWorkspacePageSize = baris per halaman. Endpoint daftar SELALU dipaginasi
// (Rule 13); platform bisa punya ribuan workspace.
const devWorkspacePageSize = 20

// DevWorkspaces — GET /dev/workspaces. Daftar semua workspace (route platform,
// lintas-tenant). Termasuk yang suspended/archived/terhapus: justru itu yang
// perlu dilihat operator, dan restore hanya mungkin bila barisnya tampak.
func (h *Handler) DevWorkspaces(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 1 {
		page = p
	}
	offset := int32((page - 1) * devWorkspacePageSize)

	rows, err := h.q(ctx).ListTenantsForPlatform(ctx, db.ListTenantsForPlatformParams{
		Limit: devWorkspacePageSize, Offset: offset,
	})
	if err != nil {
		h.Log.Error("dev workspaces: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	total, err := h.q(ctx).CountTenantsForPlatform(ctx)
	if err != nil {
		h.Log.Error("dev workspaces: count", "err", err)
		total = int64(len(rows))
	}

	items := make([]dev.WorkspaceRow, 0, len(rows))
	for _, t := range rows {
		items = append(items, dev.WorkspaceRow{
			ID: t.ID, Name: t.Name, Slug: t.Slug, Status: t.Status,
			Members: t.MemberCount,
			Deleted: t.DeletedAt.Valid,
			Reason:  strFromPtr(t.SuspendReason),
		})
	}
	h.renderShell(w, r, "Workspaces", "go_starter /dev", "/dev/workspaces", devNav(r.Context()),
		dev.Workspaces(items, page, total, devWorkspacePageSize,
			wsErrMsg(r.URL.Query().Get("err"))))
}

// DevWorkspaceSuspend — POST /dev/workspaces/{id}/suspend. Alasan WAJIB: tanpa
// itu, "kenapa workspace saya mati" jadi pertanyaan support yang tak terjawab
// oleh siapa pun, termasuk operator berikutnya.
func (h *Handler) DevWorkspaceSuspend(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	reason := strings.TrimSpace(r.FormValue("reason"))
	if reason == "" {
		http.Redirect(w, r, "/dev/workspaces?err=reason", http.StatusSeeOther)
		return
	}
	uid := session.UserID(ctx)
	if err := h.q(ctx).SuspendTenant(ctx, db.SuspendTenantParams{
		ID: id, SuspendedBy: &uid, SuspendReason: &reason,
	}); err != nil {
		h.Log.Error("dev workspaces: suspend", "tenant_id", id, "err", err)
		http.Redirect(w, r, "/dev/workspaces?err=failed", http.StatusSeeOther)
		return
	}
	h.auditWorkspace(ctx, uid, "workspace.suspend", id, map[string]string{"reason": reason})
	http.Redirect(w, r, "/dev/workspaces", http.StatusSeeOther)
}

// DevWorkspaceUnsuspend — POST /dev/workspaces/{id}/unsuspend. PLATFORM-ONLY:
// inilah yang membuat suspensi berarti sesuatu.
func (h *Handler) DevWorkspaceUnsuspend(w http.ResponseWriter, r *http.Request) {
	h.devWorkspaceAction(w, r, "workspace.unsuspend", func(id int64) error {
		return h.q(r.Context()).UnsuspendTenant(r.Context(), id)
	})
}

// DevWorkspaceRestore — POST /dev/workspaces/{id}/restore. Batalkan penghapusan
// dalam masa tenggang. Setelah lewat GracePeriodDays baris sudah dipurge dan
// query ini tak menemukan apa pun (no-op, bukan error).
func (h *Handler) DevWorkspaceRestore(w http.ResponseWriter, r *http.Request) {
	h.devWorkspaceAction(w, r, "workspace.restore", func(id int64) error {
		return h.q(r.Context()).RestoreTenant(r.Context(), id)
	})
}

// devWorkspaceAction = kerangka bersama aksi platform: parse id → jalankan →
// audit → kembali ke daftar. Tanpa ini ketiga handler mengulang blok yang sama
// dan satu yang lupa mengaudit tak akan terlihat.
func (h *Handler) devWorkspaceAction(w http.ResponseWriter, r *http.Request, action string, fn func(int64) error) {
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if err := fn(id); err != nil {
		h.Log.Error("dev workspaces: "+action, "tenant_id", id, "err", err)
		http.Redirect(w, r, "/dev/workspaces?err=failed", http.StatusSeeOther)
		return
	}
	h.audit(r.Context(), session.UserID(r.Context()), action, id, nil)
	http.Redirect(w, r, "/dev/workspaces", http.StatusSeeOther)
}

// strFromPtr = *string → string ("" bila nil). Kolom nullable dari sqlc datang
// sebagai pointer; view menerima nilai siap-render.
func strFromPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
