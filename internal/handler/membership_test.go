package handler

import (
	"testing"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// membership_test.go — perilaku inti model multi-workspace: satu user boleh
// anggota banyak workspace dgn role BERBEDA, dan middleware Scope memvalidasi
// workspace aktif terhadap keanggotaan (anti tenant-forcing).

// TestMembership_RolePerWorkspace: user yang sama bisa owner di A & member di B.
func TestMembership_RolePerWorkspace(t *testing.T) {
	env, uid := setupTest(t) // uid = owner di env.tenantID
	ctx := t.Context()

	other, err := env.q.CreateTenant(ctx, db.CreateTenantParams{Name: "Lain", Slug: "lain"})
	if err != nil {
		t.Fatalf("tenant kedua: %v", err)
	}
	if _, err := env.q.CreateMembership(ctx, db.CreateMembershipParams{
		UserID: uid, TenantID: other.ID, Role: "member",
	}); err != nil {
		t.Fatalf("membership kedua: %v", err)
	}

	ms, err := env.q.ListMembershipsByUser(ctx, uid)
	if err != nil || len(ms) != 2 {
		t.Fatalf("user harus punya 2 workspace, got %d (err=%v)", len(ms), err)
	}
	roles := map[int64]string{}
	for _, m := range ms {
		roles[m.TenantID] = m.Role
	}
	if roles[env.tenantID] != "owner" || roles[other.ID] != "member" {
		t.Errorf("role harus BEDA per workspace, got %v", roles)
	}
}

// TestCountOwnedWorkspaces_KuotaBasis: hanya workspace ber-role owner yang
// dihitung untuk kuota — diundang jadi member TIDAK memakan kuota.
func TestCountOwnedWorkspaces_KuotaBasis(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()

	n, err := env.q.CountOwnedWorkspaces(ctx, uid)
	if err != nil || n != 1 {
		t.Fatalf("awal harus punya 1 workspace milik sendiri, got %d (err=%v)", n, err)
	}
	// Diundang ke workspace orang lain sebagai member → kuota TIDAK bertambah.
	other, _ := env.q.CreateTenant(ctx, db.CreateTenantParams{Name: "Tamu", Slug: "tamu"})
	if _, err := env.q.CreateMembership(ctx, db.CreateMembershipParams{
		UserID: uid, TenantID: other.ID, Role: "member",
	}); err != nil {
		t.Fatalf("membership tamu: %v", err)
	}
	n2, err := env.q.CountOwnedWorkspaces(ctx, uid)
	if err != nil || n2 != 1 {
		t.Errorf("jadi member di workspace lain tak boleh menambah kuota, got %d", n2)
	}
}

// TestCountTenantOwners_CegahYatim: dasar guard "owner terakhir tak boleh
// diturunkan/dikeluarkan" — workspace tanpa owner = yatim.
func TestCountTenantOwners_CegahYatim(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()

	n, err := env.q.CountTenantOwners(ctx, env.tenantID)
	if err != nil || n != 1 {
		t.Fatalf("workspace seed harus punya 1 owner, got %d (err=%v)", n, err)
	}
	// Tambah owner kedua → menurunkan salah satu jadi aman.
	second := env.seedMember(t, "owner2@local", "owner", 0)
	n2, err := env.q.CountTenantOwners(ctx, env.tenantID)
	if err != nil || n2 != 2 {
		t.Errorf("harus 2 owner, got %d", n2)
	}
	_ = second
	_ = uid
}

// TestResolveActiveTenant_TolakTenantAsing: session menyimpan workspace yang
// BUKAN milik user (basi / dipaksa) → Scope harus jatuh ke workspace sah user,
// bukan menuruti session. Ini pertahanan inti anti tenant-forcing.
func TestResolveActiveTenant_TolakTenantAsing(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()

	// Workspace milik ORANG LAIN (user kita bukan anggotanya).
	asing, _ := env.q.CreateTenant(ctx, db.CreateTenantParams{Name: "Asing", Slug: "asing"})
	orang2 := env.seedMember(t, "orang2@local", "owner", asing.ID)
	_ = orang2

	env.withSession(t, uid, func(sctx sessionCtx) {
		session.SetActiveTenant(sctx.ctx, asing.ID, "Asing") // ← paksa tenant asing
		got, ok := env.h.resolveActiveTenant(sctx.ctx, uid)
		if !ok {
			t.Fatal("user punya workspace sendiri — harus tetap dapat tenant")
		}
		if got == asing.ID {
			t.Fatalf("TENANT ASING TAK BOLEH DITERIMA (kebocoran!), got %d", got)
		}
		if got != env.tenantID {
			t.Errorf("harus jatuh ke workspace sah user (%d), got %d", env.tenantID, got)
		}
	})
}

// TestResolveActiveTenant_TanpaWorkspace: user tanpa membership sama sekali →
// ok=false (Scope mengarahkan ke /workspace/new).
func TestResolveActiveTenant_TanpaWorkspace(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()

	lone, err := env.q.CreateUser(ctx, db.CreateUserParams{Email: "lone@local", PassHash: ptr("x")})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	env.withSession(t, lone.ID, func(sctx sessionCtx) {
		if _, ok := env.h.resolveActiveTenant(sctx.ctx, lone.ID); ok {
			t.Error("user tanpa workspace harus ok=false (diarahkan buat workspace)")
		}
	})
}
