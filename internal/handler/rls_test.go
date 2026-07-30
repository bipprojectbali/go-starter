package handler

import (
	"context"
	"errors"
	"os"
	"testing"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// rls_test.go — bukti isolasi RLS SESUNGGUHNYA pada jalur QUERY. Di sini pool tiap
// koneksi `SET ROLE app_rw` (NOBYPASSRLS) via AfterConnect → policy
// tenant_isolation BENAR-BENAR mengikat.
//
// Sejak 0007 runtime mencapai keadaan yang sama lewat `SET LOCAL ROLE` di dalam
// tiap transaksi (bukan lagi koneksi kedua ber-DSN sendiri); yang membuktikan
// MEKANISMENYA ada di internal/db/privileges_test.go. Yang dibuktikan di FILE INI
// adalah akibatnya: dengan hak yang sudah turun, query lintas-tenant memang tak
// mengembalikan baris.
//
// CATATAN model membership: tabel `users` kini GLOBAL (identitas lintas-workspace)
// → KELUAR dari RLS. Probe isolasi memakai `audit_logs` yang masih ber-tenant_id
// + policy. Ini tetap menguji hal yang sama: baris ber-tenant tak bocor lintas
// workspace, dan "lupa WHERE tenant_id" tetap 0 baris karena DB yang menolak.

// rlsPool membuka pool yang meng-`SET ROLE app_rw` di tiap koneksi. Skip bila
// TEST_DATABASE_URL kosong atau role app_rw tak ada (migrasi belum jalan).
func rlsPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tidak di-set; lewati test RLS")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	// SET ROLE app_rw menonaktifkan bypassrls superuser koneksi → RLS aktif.
	// Transaction-local GUC (WithTenant/WithSuper) tetap berlaku di atas role ini.
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, err := c.Exec(ctx, "SET ROLE app_rw")
		return err
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	var bypass bool
	if err := pool.QueryRow(context.Background(),
		"SELECT rolbypassrls FROM pg_roles WHERE rolname='app_rw'").Scan(&bypass); err != nil {
		pool.Close()
		t.Skipf("role app_rw tak ditemukan (migrasi?): %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedTwoTenants menyeed 2 workspace + 1 user owner masing-masing + SATU baris
// audit per workspace (probe isolasi). Pakai OWNER pool (superuser, bypass) agar
// seed tak terhalang RLS. Mengembalikan (idA, idB).
func seedTwoTenants(t *testing.T) (int64, int64) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	ownerPool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("owner pool: %v", err)
	}
	defer ownerPool.Close()
	ctx := context.Background()
	if _, err := ownerPool.Exec(ctx,
		"TRUNCATE activity_presence, audit_logs, oauth_accounts, invites, memberships, users, tenants RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	q := db.New(ownerPool)
	ta, err := q.CreateTenant(ctx, db.CreateTenantParams{Name: "TA", Slug: "ta"})
	if err != nil {
		t.Fatalf("tenant A: %v", err)
	}
	tb, err := q.CreateTenant(ctx, db.CreateTenantParams{Name: "TB", Slug: "tb"})
	if err != nil {
		t.Fatalf("tenant B: %v", err)
	}
	// users GLOBAL + membership per workspace.
	ua, err := q.CreateUser(ctx, db.CreateUserParams{Email: "a1@ta", PassHash: ptr("x")})
	if err != nil {
		t.Fatalf("user A: %v", err)
	}
	ub, err := q.CreateUser(ctx, db.CreateUserParams{Email: "b1@tb", PassHash: ptr("x")})
	if err != nil {
		t.Fatalf("user B: %v", err)
	}
	for _, m := range []db.CreateMembershipParams{
		{UserID: ua.ID, TenantID: ta.ID, Role: "owner"},
		{UserID: ub.ID, TenantID: tb.ID, Role: "owner"},
	} {
		if _, err := q.CreateMembership(ctx, m); err != nil {
			t.Fatalf("membership: %v", err)
		}
	}
	// Probe ber-RLS: satu audit log per workspace. tenant_id kini *int64 (nullable
	// sejak migrasi 00010 — FK ON DELETE SET NULL agar audit selamat saat purge).
	for _, a := range []db.CreateAuditLogParams{
		{ActorUserID: &ua.ID, Action: "seed.a", TargetType: "test", TargetID: &ua.ID, Metadata: []byte("{}"), TenantID: &ta.ID},
		{ActorUserID: &ub.ID, Action: "seed.b", TargetType: "test", TargetID: &ub.ID, Metadata: []byte("{}"), TenantID: &tb.ID},
	} {
		if _, err := q.CreateAuditLog(ctx, a); err != nil {
			t.Fatalf("audit seed: %v", err)
		}
	}
	return ta.ID, tb.ID
}

// auditActions mengembalikan action semua audit log yang TERLIHAT — sengaja TANPA
// filter tenant, jadi RLS yang menyaring.
func auditActions(ctx context.Context, q *db.Queries) ([]string, error) {
	rows, err := q.ListAuditLogs(ctx, 100)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Action)
	}
	return out, nil
}

// TestRLS_TenantIsolation: WithTenant(A) HANYA melihat baris workspace A — walau
// query TANPA WHERE tenant_id. Ini inti defense-in-depth.
func TestRLS_TenantIsolation(t *testing.T) {
	pool := rlsPool(t)
	idA, idB := seedTwoTenants(t)
	ctx := context.Background()

	var gotA []string
	if err := db.WithTenant(ctx, pool, idA, func(q *db.Queries) error {
		a, e := auditActions(ctx, q)
		gotA = a
		return e
	}); err != nil {
		t.Fatalf("WithTenant A: %v", err)
	}
	if len(gotA) != 1 || gotA[0] != "seed.a" {
		t.Fatalf("workspace A harus lihat HANYA seed.a, got %v", gotA)
	}

	var gotB []string
	if err := db.WithTenant(ctx, pool, idB, func(q *db.Queries) error {
		b, e := auditActions(ctx, q)
		gotB = b
		return e
	}); err != nil {
		t.Fatalf("WithTenant B: %v", err)
	}
	if len(gotB) != 1 || gotB[0] != "seed.b" {
		t.Fatalf("workspace B harus lihat HANYA seed.b, got %v", gotB)
	}
}

// TestRLS_SuperBypass: WithSuper melihat SEMUA workspace (jalur platform).
func TestRLS_SuperBypass(t *testing.T) {
	pool := rlsPool(t)
	seedTwoTenants(t)
	ctx := context.Background()

	var all []string
	if err := db.WithSuper(ctx, pool, func(q *db.Queries) error {
		a, e := auditActions(ctx, q)
		all = a
		return e
	}); err != nil {
		t.Fatalf("WithSuper: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("super harus lihat 2 audit (semua workspace), got %d: %v", len(all), all)
	}
}

// TestRLS_WithCheckRejectsCrossTenant: INSERT baris workspace B saat scope A
// DITOLAK oleh WITH CHECK (tak bisa menanam data ke workspace lain).
func TestRLS_WithCheckRejectsCrossTenant(t *testing.T) {
	pool := rlsPool(t)
	idA, idB := seedTwoTenants(t)
	ctx := context.Background()

	err := db.WithTenant(ctx, pool, idA, func(q *db.Queries) error {
		actor := int64(1)
		_, e := q.CreateAuditLog(ctx, db.CreateAuditLogParams{
			ActorUserID: &actor, Action: "evil", TargetType: "test",
			TargetID: &actor, Metadata: []byte("{}"), TenantID: &idB, // ← workspace lain
		})
		return e
	})
	if err == nil {
		t.Fatal("INSERT lintas-workspace HARUS ditolak WITH CHECK, tapi sukses")
	}
	if !containsRLSViolation(err) {
		t.Fatalf("error harus pelanggaran RLS, got: %v", err)
	}
}

// TestRLS_PoolLeakIsolation: request workspace A lalu B pada POOL yang sama →
// B tak melihat A. set_config(...,true) transaction-local mencegah kebocoran GUC.
func TestRLS_PoolLeakIsolation(t *testing.T) {
	pool := rlsPool(t)
	idA, idB := seedTwoTenants(t)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		var nA, nB int
		if err := db.WithTenant(ctx, pool, idA, func(q *db.Queries) error {
			a, e := auditActions(ctx, q)
			nA = len(a)
			return e
		}); err != nil {
			t.Fatalf("iter %d A: %v", i, err)
		}
		if err := db.WithTenant(ctx, pool, idB, func(q *db.Queries) error {
			b, e := auditActions(ctx, q)
			nB = len(b)
			return e
		}); err != nil {
			t.Fatalf("iter %d B: %v", i, err)
		}
		if nA != 1 || nB != 1 {
			t.Fatalf("iter %d: GUC bocor antar-peminjam pool (nA=%d nB=%d, harus 1)", i, nA, nB)
		}
	}
}

// TestRLS_NoGUCDeniesAll: tanpa GUC sama sekali → deny-default (0 baris).
// Membuktikan policy tak fail-open.
func TestRLS_NoGUCDeniesAll(t *testing.T) {
	pool := rlsPool(t)
	seedTwoTenants(t)
	ctx := context.Background()

	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM audit_logs").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("tanpa GUC, RLS harus deny-default (0 baris), got %d", n)
	}
}

// TestUsersTableIsGlobal: users KELUAR dari RLS (identitas lintas-workspace) —
// terlihat penuh walau tanpa GUC. Ini KESENGAJAAN model membership, bukan bocor:
// isolasi data ada di tabel ber-tenant_id (audit_logs dkk), dan akses workspace
// ditentukan memberships.
func TestUsersTableIsGlobal(t *testing.T) {
	pool := rlsPool(t)
	seedTwoTenants(t)
	ctx := context.Background()

	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&n); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if n != 2 {
		t.Fatalf("users global harus terlihat semua (2), got %d", n)
	}
}

// --- helper ---

// containsRLSViolation melaporkan apakah err = pelanggaran RLS Postgres.
// SQLSTATE 42501 = insufficient_privilege ("new row violates row-level security
// policy"). Cocokkan via pgconn.PgError (bukan string) agar tahan perubahan wording.
func containsRLSViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42501"
}
