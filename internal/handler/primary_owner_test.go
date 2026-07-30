package handler

import (
	"testing"

	"go_starter/internal/appmode"
	"go_starter/internal/authz"
	"go_starter/internal/db"
)

// primary_owner_test.go — keputusan 0007: super_admin sebagai OWNER workspace
// primer, dan sifat-sifat workspace itu (kebal arsip/hapus, lolos kuota).
//
// Yang dijaga di sini adalah hal yang baru terasa NANTI: selama masih single
// semuanya tampak baik-baik saja karena platform bisa apa saja. Kerusakannya
// muncul setelah mode naik ke multi — saat rumah aplikasi ternyata satu-satunya
// workspace yang tak bisa dikelola seperti workspace lain.

// seedPrimary membuat workspace primer dan mengembalikannya.
func seedPrimary(t *testing.T, env *testEnv) db.Tenant {
	t.Helper()
	tn, err := env.q.CreatePrimaryTenant(t.Context(), db.CreatePrimaryTenantParams{
		Name: "App", Slug: appmode.PrimarySlug,
	})
	if err != nil {
		t.Fatalf("seed workspace primer: %v", err)
	}
	return tn
}

// TestEnsurePrimaryOwner_MembuatDanMenaikkan: idempotent & PROMOTE-ONLY.
// Tanpa ini, workspace tempat seluruh aplikasi dibangun tak punya owner sama
// sekali — dan setelah mode naik ke multi, ia jadi satu-satunya workspace yang
// mustahil dikelola lewat panel anggota seperti yang lain.
func TestEnsurePrimaryOwner_MembuatDanMenaikkan(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	prim := seedPrimary(t, env)
	root := env.seedUserOnly(t, "root@local")

	// 1. Belum punya membership → dibuatkan sebagai owner.
	if err := env.h.ensurePrimaryOwner(ctx, env.q, root.ID); err != nil {
		t.Fatalf("ensurePrimaryOwner: %v", err)
	}
	m, err := env.q.GetMembership(ctx, db.GetMembershipParams{UserID: root.ID, TenantID: prim.ID})
	if err != nil {
		t.Fatalf("membership harus terbentuk: %v", err)
	}
	if m.Role != authz.RoleNameOwner {
		t.Errorf("harus owner, got %q", m.Role)
	}

	// 2. Idempotent — dipanggil tiap login, jadi tak boleh menumpuk atau gagal.
	if err := env.h.ensurePrimaryOwner(ctx, env.q, root.ID); err != nil {
		t.Fatalf("panggilan kedua: %v", err)
	}

	// 3. PROMOTE-ONLY: yang sudah member dinaikkan, bukan dibiarkan.
	biasa := env.seedUserOnly(t, "biasa@local")
	if _, err := env.q.CreateMembership(ctx, db.CreateMembershipParams{
		UserID: biasa.ID, TenantID: prim.ID, Role: authz.RoleNameMember,
	}); err != nil {
		t.Fatalf("seed member: %v", err)
	}
	if err := env.h.ensurePrimaryOwner(ctx, env.q, biasa.ID); err != nil {
		t.Fatalf("promote: %v", err)
	}
	m2, _ := env.q.GetMembership(ctx, db.GetMembershipParams{UserID: biasa.ID, TenantID: prim.ID})
	if m2.Role != authz.RoleNameOwner {
		t.Errorf("member harus dinaikkan jadi owner, got %q", m2.Role)
	}
}

// TestPrimaryTenant_TakBisaDiarsipkanAtauDihapus: dijaga di SQL, bukan cuma di
// handler. Guard handler menahan jalur normal; guard SQL menahan yang tidak.
// Mengarsipkan rumah aplikasi = SELURUH aplikasi jadi read-only lewat tombol
// yang tampak rutin, dan tak ada halaman tersisa untuk membatalkannya.
func TestPrimaryTenant_TakBisaDiarsipkanAtauDihapus(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	prim := seedPrimary(t, env)

	if err := env.q.ArchiveTenant(ctx, prim.ID); err != nil {
		t.Fatalf("query tak boleh error, cukup tak berefek: %v", err)
	}
	if err := env.q.SoftDeleteTenant(ctx, prim.ID); err != nil {
		t.Fatalf("query tak boleh error: %v", err)
	}

	after, err := env.q.GetTenant(ctx, prim.ID)
	if err != nil {
		t.Fatalf("baca ulang: %v", err)
	}
	if after.Status != TenantActive {
		t.Errorf("workspace primer TAK BOLEH bisa diarsipkan, status jadi %q", after.Status)
	}
	if after.DeletedAt.Valid {
		t.Error("workspace primer TAK BOLEH bisa dihapus")
	}

	// Workspace BIASA tetap bisa — guard-nya khusus yang primer, bukan pemblokiran
	// menyeluruh yang kebetulan lewat.
	if err := env.q.ArchiveTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("arsip workspace biasa: %v", err)
	}
	biasa, _ := env.q.GetTenant(ctx, env.tenantID)
	if biasa.Status != TenantArchived {
		t.Errorf("workspace biasa harus tetap bisa diarsipkan, got %q", biasa.Status)
	}
}

// TestPrimaryTenant_TakMemakanKuota: kuota membatasi berapa yang boleh DIBUAT,
// dan rumah aplikasi tak dibuat siapa pun — ia ada sebelum user pertama
// mendaftar. Tanpa pengecualian ini, super_admin yang jadi ownernya memakai
// jatahnya sendiri: dengan default 1, orang yang MENETAPKAN aturan kuota justru
// tak bisa membuat workspace apa pun. Absurd, dan baru ketahuan saat ia menekan
// tombolnya.
func TestPrimaryTenant_TakMemakanKuota(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	prim := seedPrimary(t, env)
	root := env.seedUserOnly(t, "root@local")

	if _, err := env.q.CreateMembership(ctx, db.CreateMembershipParams{
		UserID: root.ID, TenantID: prim.ID, Role: authz.RoleNameOwner,
	}); err != nil {
		t.Fatalf("seed owner primer: %v", err)
	}
	n, err := env.q.CountOwnedWorkspaces(ctx, root.ID)
	if err != nil {
		t.Fatalf("hitung kuota: %v", err)
	}
	if n != 0 {
		t.Errorf("workspace primer tak boleh memakan kuota, terhitung %d", n)
	}

	// Workspace biasa TETAP dihitung — pengecualiannya sempit.
	biasa, err := env.q.CreateTenant(ctx, db.CreateTenantParams{Name: "Punya Root", Slug: "punya-root"})
	if err != nil {
		t.Fatalf("seed workspace biasa: %v", err)
	}
	if _, err := env.q.CreateMembership(ctx, db.CreateMembershipParams{
		UserID: root.ID, TenantID: biasa.ID, Role: authz.RoleNameOwner,
	}); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	if n, _ := env.q.CountOwnedWorkspaces(ctx, root.ID); n != 1 {
		t.Errorf("workspace biasa harus tetap dihitung, got %d", n)
	}
}

// TestSidebarKuota_SamaDenganPenegakan: sidebar menghitung sisa jatah dari daftar
// keanggotaan, penegakan memakai CountOwnedWorkspaces. Dua hitungan yang berbeda
// sedikit saja membuat user melihat tombol "Buat workspace" yang lalu ditolak —
// atau kehilangan tombol yang masih jadi haknya.
func TestSidebarKuota_SamaDenganPenegakan(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	prim := seedPrimary(t, env)
	root := env.seedUserOnly(t, "root@local")

	for _, tid := range []int64{prim.ID, env.tenantID} {
		if _, err := env.q.CreateMembership(ctx, db.CreateMembershipParams{
			UserID: root.ID, TenantID: tid, Role: authz.RoleNameOwner,
		}); err != nil {
			t.Fatalf("seed membership: %v", err)
		}
	}

	// Sumber sidebar.
	rows, err := env.q.ListMembershipsByUser(ctx, root.ID)
	if err != nil {
		t.Fatalf("daftar keanggotaan: %v", err)
	}
	var owned int64
	for _, m := range rows {
		if m.Role == authz.RoleNameOwner && !m.IsPrimary {
			owned++
		}
	}
	// Sumber penegakan.
	enforced, err := env.q.CountOwnedWorkspaces(ctx, root.ID)
	if err != nil {
		t.Fatalf("hitung kuota: %v", err)
	}
	if owned != enforced {
		t.Errorf("hitungan sidebar (%d) harus sama dengan penegakan (%d)", owned, enforced)
	}
}
