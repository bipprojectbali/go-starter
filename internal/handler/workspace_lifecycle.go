package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// workspace_lifecycle.go — aksi owner atas workspace-nya sendiri: arsip, hapus,
// pulihkan (keputusan 0005). Suspend/unsuspend TIDAK di sini — itu wewenang
// platform, ada di dev_workspaces.go, dan pemisahan file ini mengikuti pemisahan
// kewenangannya.

// WorkspaceArchive — POST /w/{workspace}/archive. Owner menjadikan workspace
// hanya-baca. Guard `status='active'` ada di SQL: archive tak boleh menimpa
// suspensi platform (owner bisa keluar lewat pintu samping arsip→unarsip).
func (h *Handler) WorkspaceArchive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canEditWorkspace(ctx) {
		wsRedirect(w, r, "/settings", "forbidden")
		return
	}
	tenantID := session.TenantID(ctx)
	if err := h.q(ctx).ArchiveTenant(ctx, tenantID); err != nil {
		h.Log.Error("workspace: archive", "tenant_id", tenantID, "err", err)
		wsRedirect(w, r, "/settings", "failed")
		return
	}
	h.audit(ctx, session.UserID(ctx), "workspace.archive", tenantID, nil)
	wsRedirect(w, r, "/settings", "")
}

// WorkspaceUnarchive — POST /workspace/{workspace}/unarchive. SENGAJA di luar
// prefix /w/{slug}: gerbang read-only workspace terarsip memblokir semua POST di
// dalamnya, jadi pintu keluarnya harus berada di luar ruangan yang ia buka
// (0005 §4). Konsekuensinya route ini memvalidasi keanggotaannya sendiri.
func (h *Handler) WorkspaceUnarchive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := session.UserID(ctx)
	slug := slugFromRequest(r)

	// Pencarian tenant dan pemeriksaan OTORITAS sengaja dipisah. resolveTenantBySlug
	// mensyaratkan KEANGGOTAAN, sementara platform (super_admin/staff) bukan anggota
	// workspace mana pun — memakainya untuk keduanya membuat platform ditolak 404 di
	// sini padahal isOwnerOf di bawah justru mengizinkannya (dua cek yang saling
	// bertentangan). Platform mencari lewat jalur lintas-workspace-nya sendiri.
	var (
		t  db.Tenant
		ok bool
	)
	if isPlatformRole(session.Role(ctx)) || session.IsRoot(ctx) {
		t, ok = h.tenantBySlug(ctx, slug)
	} else {
		t, ok = h.resolveTenantBySlug(ctx, uid, slug)
	}
	if !ok || t.DeletedAt.Valid {
		http.NotFound(w, r) // tak ada / bukan anggota / terhapus — jangan bocorkan bedanya
		return
	}
	// Otoritas dibaca dari membership DI WORKSPACE ITU, bukan dari session: route
	// ini di luar Scope, jadi session.Role bisa saja milik workspace lain.
	if !h.isOwnerOf(ctx, uid, t.ID) {
		http.Error(w, "hanya owner yang dapat mengaktifkan kembali", http.StatusForbidden)
		return
	}
	if err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		return q.UnarchiveTenant(ctx, t.ID)
	}); err != nil {
		h.Log.Error("workspace: unarchive", "tenant_id", t.ID, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.audit(ctx, uid, "workspace.unarchive", t.ID, nil)
	http.Redirect(w, r, wsPath(slug, "/settings"), http.StatusSeeOther)
}

// WorkspaceDelete — POST /w/{workspace}/delete. SOFT-delete: baris tetap ada,
// slug tak dilepas (0005 §5). Setelah ini workspace hilang dari switcher, jadi
// user diantar ke luar — ke workspace lain miliknya atau form buat baru.
func (h *Handler) WorkspaceDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canEditWorkspace(ctx) {
		wsRedirect(w, r, "/settings", "forbidden")
		return
	}
	tenantID := session.TenantID(ctx)
	if err := h.q(ctx).SoftDeleteTenant(ctx, tenantID); err != nil {
		h.Log.Error("workspace: delete", "tenant_id", tenantID, "err", err)
		wsRedirect(w, r, "/settings", "failed")
		return
	}
	h.audit(ctx, session.UserID(ctx), "workspace.delete", tenantID, nil)
	// Session masih menunjuk workspace yang baru saja dihapus — kosongkan agar
	// Scope memilih ulang dari daftar yang masih hidup, bukan mencoba membukanya
	// lagi dan berakhir 404 di halaman berikutnya.
	session.SetActiveTenant(ctx, 0, "", "")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// isOwnerOf memeriksa role user DI SATU workspace tertentu — dipakai jalur di
// luar Scope, tempat session.Role belum tentu milik workspace yang dimaksud.
// Platform (super_admin/staff) selalu lolos: mereka lintas-workspace.
func (h *Handler) isOwnerOf(ctx context.Context, uid, tenantID int64) bool {
	if session.IsRoot(ctx) || isPlatformRole(session.Role(ctx)) {
		return true
	}
	var owner bool
	if err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		m, e := q.GetMembership(ctx, db.GetMembershipParams{UserID: uid, TenantID: tenantID})
		if e != nil {
			return nil // bukan anggota → bukan owner; bukan error
		}
		owner = m.Role == authz.RoleNameOwner
		return nil
	}); err != nil {
		h.Log.Error("lifecycle: cek owner", "user_id", uid, "tenant_id", tenantID, "err", err)
		return false // gagal baca → tolak (arah aman)
	}
	return owner
}
