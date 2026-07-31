package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/oauth"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// invite.go — undang orang ke workspace. Undangan dibuat owner/admin, penerima
// membuka link bertoken untuk bergabung. Email BELUM dikirim (task terbuka):
// link ditampilkan di panel anggota untuk disalin manual.
//
// TODO(invite): kirim email otomatis — butuh SMTP/provider (net/smtp stdlib,
// nol dependency). Sampai itu ada, link disalin manual dari halaman anggota.

// inviteTTL = masa berlaku undangan. Cukup lama untuk dibagikan, cukup pendek
// agar token bocor tak berlaku selamanya.
const inviteTTL = 7 * 24 * time.Hour

// InviteCreate — POST /w/{workspace}/members/invite. Buat undangan (owner/admin saja).
func (h *Handler) InviteCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageMembers(ctx) {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	role := r.FormValue("role")
	if email == "" || !strings.Contains(email, "@") {
		wsRedirect(w, r, "/members", "email")
		return
	}
	// owner TAK bisa diundang (harus diangkat setelah bergabung) — CHECK di DB juga.
	if role != authz.RoleNameAdmin && role != authz.RoleNameMember {
		role = authz.RoleNameMember
	}
	token, err := oauth.NewState() // 32-byte crypto/rand hex (dipakai ulang, nol dep)
	if err != nil {
		h.Log.Error("invite: token", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	uid := session.UserID(ctx)
	if _, err := h.q(ctx).CreateInvite(ctx, db.CreateInviteParams{
		TenantID:  session.TenantID(ctx),
		Email:     email,
		Role:      role,
		Token:     token,
		InvitedBy: &uid,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(inviteTTL), Valid: true},
	}); err != nil {
		h.Log.Error("invite: create", "err", err)
		wsRedirect(w, r, "/members", "failed")
		return
	}
	// metadata TANPA PII: email undangan tak dicatat, hanya role.
	h.auditWorkspace(ctx, uid, "invite.create", session.TenantID(ctx), map[string]string{"role": role})
	wsRedirect(w, r, "/members", "")
}

// InviteDelete — POST /w/{workspace}/members/invite/{id}/delete. Batalkan undangan.
func (h *Handler) InviteDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageMembers(ctx) {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		wsRedirect(w, r, "/members", "notfound")
		return
	}
	if err := h.q(ctx).DeleteInvite(ctx, db.DeleteInviteParams{
		ID: id, TenantID: session.TenantID(ctx), // filter tenant: tak bisa hapus milik orang lain
	}); err != nil {
		h.Log.Error("invite: delete", "err", err)
	}
	wsRedirect(w, r, "/members", "")
}

// InvitePage — GET /invite/{token}. PUBLIK: penerima belum tentu login (atau
// belum punya akun). Belum login → simpan token di session, arahkan ke register.
// Sudah login → halaman konfirmasi "Gabung ke workspace X?".
func (h *Handler) InvitePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := chi.URLParam(r, "token")

	inv, err := h.loadInvite(ctx, token)
	if err != nil {
		h.renderPage(w, r, "Undangan", panel.InviteResult("", inviteErrText(err), false))
		return
	}
	if session.UserID(ctx) == 0 {
		// Belum login: ingat token agar setelah register/login otomatis diterima.
		session.PutPendingInvite(ctx, token)
		if err := session.WriteCookie(ctx, w); err != nil {
			h.Log.Error("invite: write cookie", "err", err)
		}
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}
	h.renderPage(w, r, "Undangan", panel.InviteResult(inv.TenantName, "", true))
}

// InviteAccept — POST /invite/{token}/accept. Butuh login. Buat membership +
// tandai undangan terpakai, lalu pindah ke workspace itu.
func (h *Handler) InviteAccept(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := chi.URLParam(r, "token")
	uid := session.UserID(ctx)
	if uid == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := h.acceptInvite(ctx, token, uid); err != nil {
		h.renderPage(w, r, "Undangan", panel.InviteResult("", inviteErrText(err), false))
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
