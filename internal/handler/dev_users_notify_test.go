package handler

import (
	"encoding/json"
	"net/url"
	"testing"

	"go_starter/internal/db"
)

// dev_users_notify_test.go — kabar untuk yang wewenangnya diubah dari panel
// platform.
//
// Perubahan role bisa datang dari DUA pintu: /w/{slug}/members (pengelola
// workspace) dan /dev/users (operator platform). Efeknya di sisi penerima
// IDENTIK — yang boleh ia lakukan berubah. Test-test di sini mengunci bahwa
// kabarnya tak bergantung pintu mana yang kebetulan dipakai; ketimpangan seperti
// itu tak menghasilkan error di mana pun, jadi hanya test yang bisa menahannya.

// TestDevUserSetRole_MemberiTahuYangBersangkutan: sisi utama.
func TestDevUserSetRole_MemberiTahuYangBersangkutan(t *testing.T) {
	env, superID, targetID := setupDevUsers(t)

	env.doDevAction(superID, targetID, url.Values{
		"role": {"admin"}, "tenant": {itoa(env.tenantID)},
	}, env.h.DevUserSetRole)

	rows := env.firstPage(t, targetID)
	if len(rows) != 1 {
		t.Fatalf("target harus menerima 1 notifikasi, got %d", len(rows))
	}
	if rows[0].Kind != "member.role.changed" {
		t.Errorf("kind = %q, want member.role.changed", rows[0].Kind)
	}
	// Role BARU ikut dibawa: tanpa itu kalimatnya jatuh ke "role Anda diubah"
	// tanpa menyebut jadi apa — dan penerimanya harus menebak atau bertanya.
	var p notifPayload
	if err := json.Unmarshal(rows[0].Payload, &p); err != nil {
		t.Fatalf("payload tak terbaca: %v", err)
	}
	if p.Role != "admin" {
		t.Errorf("payload.role = %q, want admin — pesan harus menyebut role barunya", p.Role)
	}
}

// TestDevUserSetRole_NotifikasiMenunjukWorkspaceTarget: panel /dev bersifat
// LINTAS-workspace, jadi workspace yang sedang dibuka aktor sering BUKAN
// workspace tempat role itu berubah. Notifikasi harus menunjuk workspace TARGET
// — kalau ia menunjuk workspace aktor, penerimanya membaca kabar tentang tempat
// yang mungkin tak pernah ia masuki.
func TestDevUserSetRole_NotifikasiMenunjukWorkspaceTarget(t *testing.T) {
	env, superID, _ := setupDevUsers(t)
	ctx := t.Context()

	// Workspace kedua + target yang jadi anggotanya. Aktor tetap "berada" di
	// workspace seed (session di doDevAction menyetel tenant 1).
	lain, err := env.q.CreateTenant(ctx, db.CreateTenantParams{Name: "Lain", Slug: "lain"})
	if err != nil {
		t.Fatalf("seed tenant kedua: %v", err)
	}
	target := env.seedMember(t, "diworkspacelain@local", "member", lain.ID)

	env.doDevAction(superID, target.ID, url.Values{
		"role": {"admin"}, "tenant": {itoa(lain.ID)},
	}, env.h.DevUserSetRole)

	rows := env.firstPage(t, target.ID)
	if len(rows) != 1 {
		t.Fatalf("target harus menerima 1 notifikasi, got %d", len(rows))
	}
	if rows[0].TenantID == nil {
		t.Fatal("notifikasi harus menyebut workspace — tanpa itu penerimanya tak tahu tempat mana")
	}
	if *rows[0].TenantID != lain.ID {
		t.Errorf("notifikasi menunjuk workspace %d, want %d (workspace TARGET, bukan workspace aktor)",
			*rows[0].TenantID, lain.ID)
	}
}

// TestDevUserSetRole_DitolakTakMemberiKabar: notifikasi menyusul PERUBAHAN, bukan
// percobaan. Kabar tentang perubahan yang tak terjadi lebih buruk daripada tak
// ada kabar — penerimanya akan mengira wewenangnya berubah lalu bingung.
func TestDevUserSetRole_DitolakTakMemberiKabar(t *testing.T) {
	env, superID, _ := setupDevUsers(t)

	// Target = root env → GuardSetRole menolak (ErrProtectedRoot).
	env.doDevAction(superID, superID, url.Values{
		"role": {"member"}, "tenant": {itoa(env.tenantID)},
	}, env.h.DevUserSetRole)

	if rows := env.firstPage(t, superID); len(rows) != 0 {
		t.Errorf("aksi yang DITOLAK tak boleh memberi kabar, got %d notifikasi", len(rows))
	}
}

// TestDevUserSetStatus_TakMemberiKabar: sisi sebaliknya, dan ini SENGAJA.
// Status menutup pintu login, jadi notifikasi in-app tak akan pernah terbaca
// oleh yang di-disable/block — ia hanya menumpuk untuk saat statusnya dipulihkan,
// yaitu ketika kabarnya sudah basi. Dikunci agar keputusan ini tak diam-diam
// terbalik oleh yang mengira ini kelalaian.
func TestDevUserSetStatus_TakMemberiKabar(t *testing.T) {
	env, superID, targetID := setupDevUsers(t)

	env.doDevAction(superID, targetID, url.Values{"status": {"blocked"}}, env.h.DevUserSetStatus)

	if rows := env.firstPage(t, targetID); len(rows) != 0 {
		t.Errorf("perubahan status tak memberi kabar in-app (tak akan terbaca), got %d", len(rows))
	}
}
