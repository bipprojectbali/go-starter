package handler

import (
	"net/http"

	"go_starter/internal/appmode"
	"go_starter/internal/authz"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// members_page.go — HALAMAN daftar anggota (baca). Dipisah dari members.go yang
// berisi AKSI (ubah role, keluarkan): halaman ini punya aturan sendiri soal apa
// yang boleh DILIHAT, dan aturan itu tumbuh bersama kebijakan privasi — bukan
// bersama daftar aksinya.

// MembersPage — GET /w/{workspace}/members. Daftar anggota + undangan pending.
//
// Terbuka untuk SEMUA anggota, dan itu keputusan: tahu siapa saja yang punya
// akses adalah bagian dari mempercayai ruang bersama — kalau daftarnya
// disembunyikan, anggota tak bisa tahu siapa yang dapat membaca pekerjaannya.
// Yang membedakan role bukan AKSES ke halaman ini, melainkan apa yang boleh
// dilakukan & dilihat di dalamnya (pola 0004: beda role = beda AKSI, bukan beda
// ALAMAT).
//
// EMAIL DISAMARKAN bagi yang tak mengelola. Itu satu-satunya PII di halaman ini,
// dan anggota biasa tak punya alasan operasional untuk alamat lengkap rekannya —
// sementara pengelola justru membutuhkannya (mengundang, mencocokkan orang).
// Penyamaran dilakukan DI SINI, bukan di view: dengan begitu email asli tak
// pernah sampai ke browser yang tak berhak, tempat ia terbaca di source meski
// tak tampak di layar.
func (h *Handler) MembersPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := session.TenantID(ctx)
	rows, err := h.q(ctx).ListMembersByTenant(ctx, tenantID)
	if err != nil {
		h.Log.Error("members: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Dihitung SEKALI di luar loop: nilainya sama untuk semua baris, dan
	// canManageMembers membaca session tiap kali dipanggil.
	canManage := canManageMembers(ctx) && !IsReadOnly(ctx)

	members := make([]panel.MemberRow, 0, len(rows))
	for _, m := range rows {
		avatar := ""
		if m.AvatarUrl != nil {
			avatar = *m.AvatarUrl
		}
		// Diri sendiri SELALU utuh: menyamarkan email orang dari dirinya sendiri
		// hanya membuat ia mengira melihat baris orang lain.
		email := m.Email
		if !canManage && m.UserID != session.UserID(ctx) {
			email = maskEmail(email)
		}
		members = append(members, panel.MemberRow{
			UserID: m.UserID, Email: email, Role: m.Role,
			AvatarURL: avatar, Status: m.Status,
		})
	}
	// Undangan pending (fail-soft: gagal → daftar kosong, halaman tetap tampil).
	invites := []panel.InviteRow{}
	if inv, e := h.q(ctx).ListInvitesByTenant(ctx, tenantID); e == nil {
		for _, i := range inv {
			invites = append(invites, panel.InviteRow{
				ID: i.ID, Email: i.Email, Role: i.Role,
				Link: inviteLink(r, i.Token), Expires: fmtLocal(i.ExpiresAt),
			})
		}
	} else {
		h.Log.Error("members: list invites", "err", e)
	}

	h.renderWorkspaceShell(w, r, "Anggota", "/members",
		panel.Members(wsPath(slugFromRequest(r), ""), authz.AssignableRoles(appmode.IsSingle()),
			members, invites, canManage,
			session.UserID(ctx), wsErrMsg(r.URL.Query().Get("err"))))
}
