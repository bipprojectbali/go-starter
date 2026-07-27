package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// invite_service.go — logika undangan (validasi token, terima, auto-accept).
// Dipisah dari invite.go (handler HTTP) agar tiap file di bawah batas 150 baris.

var (
	errInviteNotFound = errors.New("undangan tidak ditemukan")
	errInviteExpired  = errors.New("undangan kedaluwarsa")
	errInviteUsed     = errors.New("undangan sudah dipakai")
)

// loadInvite mengambil + memvalidasi undangan. WithSuper: jalur publik, penerima
// belum tentu anggota workspace mana pun (tak ada scope RLS yang relevan).
func (h *Handler) loadInvite(ctx context.Context, token string) (db.GetInviteByTokenRow, error) {
	var inv db.GetInviteByTokenRow
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		row, e := q.GetInviteByToken(ctx, token)
		inv = row
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return inv, errInviteNotFound
		}
		return inv, err
	}
	if inv.AcceptedAt.Valid {
		return inv, errInviteUsed
	}
	if inv.ExpiresAt.Valid && inv.ExpiresAt.Time.Before(time.Now()) {
		return inv, errInviteExpired
	}
	return inv, nil
}

// acceptInvite membuat membership + menandai undangan terpakai dalam SATU tx
// (atomik). AcceptInvite ber-guard accepted_at IS NULL → klik ganda tak menghasilkan
// dua membership; UNIQUE(user,tenant) jadi jaring kedua.
func (h *Handler) acceptInvite(ctx context.Context, token string, uid int64) error {
	inv, err := h.loadInvite(ctx, token)
	if err != nil {
		return err
	}
	name := inv.TenantName
	tenantID := inv.TenantID
	if err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		if _, e := q.CreateMembership(ctx, db.CreateMembershipParams{
			UserID: uid, TenantID: tenantID, Role: inv.Role,
		}); e != nil {
			return e
		}
		return q.AcceptInvite(ctx, token)
	}); err != nil {
		return err
	}
	session.SetActiveTenant(ctx, tenantID, name)
	session.ClearPendingInvite(ctx)
	h.audit(ctx, uid, "invite.accept", tenantID, map[string]string{"role": inv.Role})
	return nil
}

// acceptPendingInvite menerima undangan yang tersimpan di session — dipanggil
// SETELAH register/login sukses. Skenario: orang membuka tautan undangan sebelum
// punya akun → token disimpan → begitu akun jadi, ia langsung masuk workspace
// pengundang (tak perlu klik tautan dua kali). FAIL-SOFT: undangan kedaluwarsa
// atau invalid TIDAK menggagalkan login — user tetap masuk workspace sendiri.
func (h *Handler) acceptPendingInvite(r *http.Request) {
	ctx := r.Context()
	token := session.PendingInvite(ctx)
	if token == "" {
		return
	}
	session.ClearPendingInvite(ctx) // one-shot: jangan coba lagi tiap login
	if err := h.acceptInvite(ctx, token, session.UserID(ctx)); err != nil {
		h.Log.Warn("invite: auto-accept gagal", "err", err)
	}
}

// inviteErrText memetakan error undangan ke pesan untuk user.
func inviteErrText(err error) string {
	switch {
	case errors.Is(err, errInviteNotFound):
		return "Undangan tidak ditemukan atau tautannya salah."
	case errors.Is(err, errInviteExpired):
		return "Undangan sudah kedaluwarsa. Minta undangan baru."
	case errors.Is(err, errInviteUsed):
		return "Undangan ini sudah dipakai."
	default:
		return "Terjadi kesalahan memproses undangan."
	}
}

// inviteLink merakit URL undangan absolut dari request (skema + host). Email
// BELUM dikirim (task terbuka) — link ditampilkan di UI untuk disalin manual.
func inviteLink(r *http.Request, token string) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/invite/" + token
}
