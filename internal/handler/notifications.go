package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/go-chi/chi/v5"
)

// notifications.go — halaman notifikasi user (umpan peristiwa + undangan masuk).
//
// SEMUA akses di sini lewat db.WithSuper, BUKAN h.q(ctx). Alasan: notifikasi
// milik USER dan lintas-workspace, sementara h.q ter-scope ke satu tenant aktif
// — undangan justru datang dari workspace yang BELUM jadi milik user, jadi akan
// tersaring habis. Konsekuensinya isolasi antar-user bergantung sepenuhnya pada
// klausa WHERE user_id/email di query (lihat queries/notifications.sql).

// notifPageSize membatasi halaman pertama umpan notifikasi (Rule 13: list wajib
// terpaginasi). Keyset cursor mengikuti pola ListUsers.
const notifPageSize = 20

// NotificationsPage — GET /notifications. Menggabungkan dua sumber: peristiwa
// (tabel notifications) + undangan pending (tabel invites, sumber kebenarannya
// tetap di sana — tak diduplikasi agar terima/kedaluwarsa tak perlu disinkronkan
// di dua tempat). Membuka halaman menandai peristiwa terbaca; undangan TIDAK
// (ia tugas, bukan kabar — lihat MarkNotificationsRead).
func (h *Handler) NotificationsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := session.UserID(ctx)
	email := normalizeEmail(session.Email(ctx))

	var (
		events  []panel.NotifRow
		invites []panel.NotifInviteRow
	)
	// Fail-soft: gagal baca salah satu sumber tak boleh mengosongkan halaman
	// diam-diam tanpa jejak — error dicatat, bagian yang berhasil tetap tampil.
	if err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		cursorAt, cursorID := firstPageCursor()
		rows, err := q.ListNotifications(ctx, db.ListNotificationsParams{
			UserID: uid, CursorCreatedAt: cursorAt, CursorID: cursorID,
			PageSize: notifPageSize,
		})
		if err != nil {
			return err
		}
		events = buildNotifRows(rows)

		inv, err := q.ListPendingInvitesByEmail(ctx, email)
		if err != nil {
			return err
		}
		invites = buildNotifInvites(inv)

		// Auto-read SETELAH daftar terbaca, agar tanda "baru" pada render ini
		// masih terlihat user.
		return q.MarkNotificationsRead(ctx, uid)
	}); err != nil {
		h.Log.Error("notifications: load", "err", err)
	}

	h.renderShell(w, r, "Notifikasi", brandFor(ctx), "/notifications", navFor(ctx),
		panel.Notifications(invites, events, notifErrMsg(r.URL.Query().Get("err"))))
}

// NotificationAccept — POST /notifications/invite/{token}/accept. Memakai ulang
// acceptInvite (invite_service.go) — jalur terima yang sama dengan /invite/{token},
// termasuk guard one-time & pindah workspace aktif.
func (h *Handler) NotificationAccept(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.Redirect(w, r, "/notifications?err=notfound", http.StatusSeeOther)
		return
	}
	if err := h.acceptInvite(r.Context(), token, session.UserID(r.Context())); err != nil {
		h.Log.Warn("notifications: accept invite", "err", err)
		http.Redirect(w, r, "/notifications?err="+inviteErrCode(err), http.StatusSeeOther)
		return
	}
	// Undangan diterima → user kini di workspace itu; antar ke berandanya.
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// NotificationDecline — POST /notifications/invite/{token}/decline. DeclineInvite
// mengunci token DAN email penerima: pemegang token tak bisa menolak undangan
// milik orang lain.
func (h *Handler) NotificationDecline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := chi.URLParam(r, "token")
	if token == "" {
		http.Redirect(w, r, "/notifications?err=notfound", http.StatusSeeOther)
		return
	}
	email := normalizeEmail(session.Email(ctx))
	if err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		return q.DeclineInvite(ctx, db.DeclineInviteParams{Token: token, Email: email})
	}); err != nil {
		h.Log.Error("notifications: decline invite", "err", err)
		http.Redirect(w, r, "/notifications?err=failed", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/notifications", http.StatusSeeOther)
}

// normalizeEmail menyamakan bentuk email sebelum dibandingkan dengan
// lower(email) di SQL (pola sama auth.go/invite.go).
func normalizeEmail(s string) string { return strings.TrimSpace(strings.ToLower(s)) }

// notifErrMsg memetakan kode PRG (?err=) ke pesan untuk user.
func notifErrMsg(code string) string {
	switch code {
	case "notfound":
		return "Undangan tidak ditemukan atau tautannya salah."
	case "expired":
		return "Undangan sudah kedaluwarsa. Minta undangan baru."
	case "used":
		return "Undangan ini sudah dipakai."
	case "failed":
		return "Gagal memproses undangan."
	default:
		return ""
	}
}
