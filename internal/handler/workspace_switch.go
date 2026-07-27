package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// workspace_switch.go — pindah & buat workspace (model membership: satu user bisa
// anggota banyak workspace). Dipisah dari workspace.go (pengaturan) agar tiap file
// tetap di bawah batas handler 150 baris.

// WorkspaceSwitch — POST /workspace/switch. Pindah workspace aktif. WAJIB
// memvalidasi keanggotaan: tenant datang dari form (user-controlled), tanpa cek
// ini user bisa pindah ke workspace orang lain.
func (h *Handler) WorkspaceSwitch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, err := strconv.ParseInt(r.FormValue("tenant"), 10, 64)
	if err != nil || tenantID == 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	uid := session.UserID(ctx)
	m, err := h.q(ctx).GetMembership(ctx, db.GetMembershipParams{UserID: uid, TenantID: tenantID})
	if err != nil {
		// Bukan anggota → jangan bocorkan keberadaan workspace; diamkan ke home.
		h.Log.Warn("workspace switch: bukan anggota", "user_id", uid, "tenant_id", tenantID)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	name := ""
	if t, e := h.q(ctx).GetTenant(ctx, tenantID); e == nil {
		name = t.Name
	}
	session.SetActiveTenant(ctx, tenantID, name)
	// Role berubah antar-workspace → home bisa beda (owner→/admin, member→/user).
	http.Redirect(w, r, authz.HomePathFor(m.Role), http.StatusSeeOther)
}

// WorkspaceNewPage — GET /workspace/new. Form buat workspace baru. Juga jadi
// tujuan Scope saat user tak punya workspace sama sekali.
func (h *Handler) WorkspaceNewPage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "Workspace Baru",
		panel.WorkspaceNew(workspaceErrMsg(r.URL.Query().Get("err"))))
}

// WorkspaceCreate — POST /workspace/new. Buat workspace + membership owner
// (atomik), cek KUOTA dulu. PRG: gagal → ?err=CODE.
func (h *Handler) WorkspaceCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" || len(name) > maxWorkspaceNameLen {
		http.Redirect(w, r, "/workspace/new?err=name", http.StatusSeeOther)
		return
	}
	uid := session.UserID(ctx)

	// Route ini TANPA middleware Scope (user mungkin belum punya workspace sama
	// sekali → tak ada tenant untuk di-scope). Semua query lewat WithSuper.
	var (
		newID     int64
		overQuota bool
	)
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		// Kuota: berapa workspace yang sudah DIMILIKI (role owner) vs jatah user.
		u, e := q.GetUser(ctx, uid)
		if e != nil {
			return e
		}
		owned, e := q.CountOwnedWorkspaces(ctx, uid)
		if e != nil {
			return e
		}
		if owned >= int64(u.WorkspaceQuota) {
			overQuota = true
			return nil
		}
		// Tenant + membership dalam SATU tx → atomik (tak ada workspace yatim).
		slug, e := uniqueSlug(ctx, q, slugify(name))
		if e != nil {
			return e
		}
		t, e := q.CreateTenant(ctx, db.CreateTenantParams{Name: name, Slug: slug})
		if e != nil {
			return e
		}
		newID = t.ID
		_, e = q.CreateMembership(ctx, db.CreateMembershipParams{
			UserID: uid, TenantID: t.ID, Role: authz.RoleNameOwner,
		})
		return e
	})
	if err != nil {
		h.Log.Error("workspace create", "err", err)
		http.Redirect(w, r, "/workspace/new?err=failed", http.StatusSeeOther)
		return
	}
	if overQuota {
		http.Redirect(w, r, "/workspace/new?err=quota", http.StatusSeeOther)
		return
	}
	// Langsung pindah ke workspace baru (user jadi owner di sana).
	session.SetActiveTenant(ctx, newID, name)
	h.audit(ctx, uid, "workspace.create", newID, nil)
	http.Redirect(w, r, authz.HomePath(authz.RoleOwner), http.StatusSeeOther)
}

// workspaceErrMsg memetakan kode ?err= ke pesan (pola PRG, sama seperti authErrMsg).
func workspaceErrMsg(code string) string {
	switch code {
	case "name":
		return "Nama workspace wajib diisi (maks 60 karakter)"
	case "quota":
		return "Kuota workspace Anda sudah penuh"
	case "failed":
		return "Gagal membuat workspace"
	default:
		return ""
	}
}
