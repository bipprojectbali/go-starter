package handler

import (
	"context"
	"errors"

	"go_starter/internal/authz"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5"
)

// primary_owner.go — super_admin sebagai OWNER workspace primer (keputusan 0007).
//
// Kenapa ini ada: super_admin punya nol baris DB (0002) — ia bekerja lewat
// WithSuper, tanpa keanggotaan di mana pun. Di mode single itu berarti workspace
// tempat SELURUH aplikasi dibangun tak punya owner sama sekali. Selama masih
// single hal itu tak terasa (platform bisa apa saja), tapi begitu mode naik ke
// multi, rumah aplikasi menjadi satu-satunya workspace yang mustahil dikelola
// seperti workspace lain — wasitnya tak pernah bisa berhenti bermain.
//
// Dengan baris membership, dua topi itu TERPISAH: kepemilikan workspace primer
// (bidang tenant) dan otoritas platform (bidang env). Masing-masing bisa
// dilepas sendiri — super_admin bisa menyerahkan /w/app ke orang lain lewat
// panel anggota dan mundur jadi wasit murni, tanpa menyentuh SUPER_ADMIN_EMAILS.
//
// Ini TIDAK melanggar 0002. Yang dilarang naik dari DB adalah otoritas PLATFORM;
// itu tetap env-only. Baris di sini hanya menambah kepemilikan tenant.
//
// KONSEKUENSI YANG DISENGAJA: menghapus email dari SUPER_ADMIN_EMAILS tak lagi
// mencabut kepemilikan atas workspace primer — orangnya turun jadi owner biasa,
// bukan user biasa. Itu lebih jujur: mencabut orang dari sebuah workspace adalah
// tindakan yang harus terlihat di panel anggota (dan terekam audit), bukan efek
// samping menyunting file env.

// ensurePrimaryOwner memastikan super_admin (env) terdaftar sebagai owner
// workspace primer. Idempotent & PROMOTE-ONLY: yang sudah owner tak disentuh,
// yang belum punya membership dibuatkan, yang sudah member/admin dinaikkan.
//
// Dipanggil saat LOGIN, bukan saat boot — saat boot belum ada satu pun baris
// users, jadi tak ada yang bisa dijadikan owner. Login adalah titik paling awal
// di mana orangnya benar-benar ada.
//
// Fail-soft: gagal di sini tak boleh menggagalkan login. Kehilangan kepemilikan
// sementara jauh lebih ringan daripada super_admin yang tak bisa masuk sama
// sekali — dan panggilan berikutnya akan memperbaikinya.
func (h *Handler) ensurePrimaryOwner(ctx context.Context, q *db.Queries, userID int64) error {
	t, err := q.GetPrimaryTenant(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Dibuat saat boot (BootstrapPrimary), jadi absennya = bug wiring.
			// Tetap tak digagalkan: pemanggil memperlakukan ini fail-soft.
			return err
		}
		return err
	}

	m, err := q.GetMembership(ctx, db.GetMembershipParams{UserID: userID, TenantID: t.ID})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		_, err = q.CreateMembership(ctx, db.CreateMembershipParams{
			UserID: userID, TenantID: t.ID, Role: authz.RoleNameOwner,
		})
		return err
	case err != nil:
		return err
	case m.Role == authz.RoleNameOwner:
		return nil // sudah benar
	default:
		// PROMOTE-ONLY: hanya menaikkan. Kalau kelak super_admin sengaja
		// diturunkan jadi member di workspace primer, ia akan naik lagi setiap
		// login — itu memang yang diinginkan selama emailnya masih di env, dan
		// jalan keluarnya adalah mencabutnya dari env, bukan menurunkan role.
		return q.UpdateMemberRole(ctx, db.UpdateMemberRoleParams{
			UserID: userID, TenantID: t.ID, Role: authz.RoleNameOwner,
		})
	}
}
