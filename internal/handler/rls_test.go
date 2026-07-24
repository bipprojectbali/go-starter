package handler

import (
	"context"
	"errors"
	"os"
	"testing"

	"go_stater/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// listAll = ListUsers halaman pertama penuh (cursor maks). Dipakai test RLS untuk
// query TANPA filter tenant eksplisit — RLS yang menyaring.
func listAll(ctx context.Context, q *db.Queries) ([]db.User, error) {
	at, id := firstPageCursor()
	return q.ListUsers(ctx, db.ListUsersParams{CursorCreatedAt: at, CursorID: id, PageSize: 100})
}

// rls_test.go — bukti isolasi RLS SESUNGGUHNYA. Test lain konek superuser (bypass
// RLS diam-diam, cuma uji logika). Di sini pool tiap koneksi `SET ROLE app_rw`
// (NOBYPASSRLS) via AfterConnect → policy tenant_isolation BENAR-BENAR mengikat,
// persis seperti runtime produksi (APP_DATABASE_URL = role app_rw).
//
// Ini test paling penting untuk template: membuktikan "lupa WHERE tenant_id"
// tetap 0 baris (DB yang menolak), bukan kebocoran.

// rlsPool membuka pool yang meng-`SET ROLE app_rw` di tiap koneksi. Skip bila
// TEST_DATABASE_URL kosong atau role app_rw tak ada (migrasi 00007 belum jalan).
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
	// SET ROLE app_rw menonaktifkan bypassrls milik superuser koneksi → RLS aktif.
	// Transaction-local GUC (WithTenant/WithSuper) tetap berlaku di atas role ini.
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, err := c.Exec(ctx, "SET ROLE app_rw")
		return err
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	// Pastikan app_rw benar-benar non-bypass (kalau migrasi belum jalan → skip).
	var bypass bool
	if err := pool.QueryRow(context.Background(),
		"SELECT rolbypassrls FROM pg_roles WHERE rolname='app_rw'").Scan(&bypass); err != nil {
		pool.Close()
		t.Skipf("role app_rw tak ditemukan (migrasi 00007?): %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedTwoTenants menyeed 2 tenant + 1 user owner masing-masing, memakai OWNER
// pool (superuser, bypass) agar seed tak terhalang RLS. Mengembalikan (idA, idB).
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
		"TRUNCATE activity_presence, audit_logs, oauth_accounts, users, tenants RESTART IDENTITY CASCADE"); err != nil {
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
	if _, err := q.CreateUser(ctx, db.CreateUserParams{Email: "a1@ta", PassHash: ptr("x"), TenantID: ta.ID, Role: "owner"}); err != nil {
		t.Fatalf("user A: %v", err)
	}
	if _, err := q.CreateUser(ctx, db.CreateUserParams{Email: "b1@tb", PassHash: ptr("x"), TenantID: tb.ID, Role: "owner"}); err != nil {
		t.Fatalf("user B: %v", err)
	}
	return ta.ID, tb.ID
}

// TestRLS_TenantIsolation: WithTenant(A) HANYA melihat baris tenant A — walau
// query TANPA WHERE tenant_id. Ini inti defense-in-depth.
func TestRLS_TenantIsolation(t *testing.T) {
	pool := rlsPool(t)
	idA, idB := seedTwoTenants(t)
	ctx := context.Background()

	// Sebagai tenant A: ListUsers (tak ada filter tenant di query) → hanya A.
	var gotA []db.User
	if err := db.WithTenant(ctx, pool, idA, func(q *db.Queries) error {
		u, e := listAll(ctx, q)
		gotA = u
		return e
	}); err != nil {
		t.Fatalf("WithTenant A: %v", err)
	}
	if len(gotA) != 1 || gotA[0].Email != "a1@ta" {
		t.Fatalf("tenant A harus lihat HANYA a1@ta, got %+v", emails(gotA))
	}

	// Sebagai tenant B: hanya B.
	var gotB []db.User
	if err := db.WithTenant(ctx, pool, idB, func(q *db.Queries) error {
		u, e := listAll(ctx, q)
		gotB = u
		return e
	}); err != nil {
		t.Fatalf("WithTenant B: %v", err)
	}
	if len(gotB) != 1 || gotB[0].Email != "b1@tb" {
		t.Fatalf("tenant B harus lihat HANYA b1@tb, got %+v", emails(gotB))
	}
}

// TestRLS_SuperBypass: WithSuper melihat SEMUA tenant (jalur platform).
func TestRLS_SuperBypass(t *testing.T) {
	pool := rlsPool(t)
	seedTwoTenants(t)
	ctx := context.Background()

	var all []db.User
	if err := db.WithSuper(ctx, pool, func(q *db.Queries) error {
		u, e := listAll(ctx, q)
		all = u
		return e
	}); err != nil {
		t.Fatalf("WithSuper: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("super harus lihat 2 user (semua tenant), got %d: %v", len(all), emails(all))
	}
}

// TestRLS_WithCheckRejectsCrossTenant: INSERT baris tenant B saat scope tenant A
// DITOLAK oleh WITH CHECK (tak bisa menanam data ke tenant lain).
func TestRLS_WithCheckRejectsCrossTenant(t *testing.T) {
	pool := rlsPool(t)
	idA, idB := seedTwoTenants(t)
	ctx := context.Background()

	err := db.WithTenant(ctx, pool, idA, func(q *db.Queries) error {
		// Coba buat user di tenant B sementara scope = A → WITH CHECK menolak.
		_, e := q.CreateUser(ctx, db.CreateUserParams{
			Email: "evil@tb", PassHash: ptr("x"), TenantID: idB, Role: "member",
		})
		return e
	})
	if err == nil {
		t.Fatal("INSERT lintas-tenant (B saat scope A) HARUS ditolak WITH CHECK, tapi sukses")
	}
	// Pastikan pesan RLS (row-level security), bukan error lain.
	if !containsRLSViolation(err) {
		t.Fatalf("error harus pelanggaran RLS, got: %v", err)
	}
}

// TestRLS_PoolLeakIsolation: request tenant A lalu tenant B pada POOL yang sama
// (koneksi bisa dipakai ulang) → B tak melihat A. set_config(...,true)
// transaction-local mencegah kebocoran GUC antar-peminjam pool.
func TestRLS_PoolLeakIsolation(t *testing.T) {
	pool := rlsPool(t)
	idA, idB := seedTwoTenants(t)
	ctx := context.Background()

	// Banyak iterasi bergantian A/B menaikkan peluang reuse koneksi yang sama.
	for i := 0; i < 10; i++ {
		var nA int
		if err := db.WithTenant(ctx, pool, idA, func(q *db.Queries) error {
			u, e := listAll(ctx, q)
			nA = len(u)
			return e
		}); err != nil {
			t.Fatalf("iter %d A: %v", i, err)
		}
		var nB int
		if err := db.WithTenant(ctx, pool, idB, func(q *db.Queries) error {
			u, e := listAll(ctx, q)
			nB = len(u)
			return e
		}); err != nil {
			t.Fatalf("iter %d B: %v", i, err)
		}
		if nA != 1 || nB != 1 {
			t.Fatalf("iter %d: GUC bocor antar-peminjam pool (nA=%d nB=%d, keduanya harus 1)", i, nA, nB)
		}
	}
}

// TestRLS_NoGUCDeniesAll: tanpa GUC tenant sama sekali (bukan WithTenant/WithSuper)
// → deny-default (0 baris). Membuktikan policy tak fail-open.
func TestRLS_NoGUCDeniesAll(t *testing.T) {
	pool := rlsPool(t)
	seedTwoTenants(t)
	ctx := context.Background()

	// Query langsung tanpa set_config apa pun (role app_rw, RLS aktif) → 0 baris.
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("tanpa GUC tenant, RLS harus deny-default (0 baris), got %d", n)
	}
}

// --- helper ---

func emails(us []db.User) []string {
	out := make([]string, len(us))
	for i, u := range us {
		out[i] = u.Email
	}
	return out
}

// containsRLSViolation melaporkan apakah err = pelanggaran RLS Postgres.
// SQLSTATE 42501 = insufficient_privilege (dipakai untuk "new row violates
// row-level security policy" pada WITH CHECK). Cocokkan via pgconn.PgError
// (bukan string) agar tahan perubahan wording.
func containsRLSViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42501"
}
