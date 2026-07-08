package database

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMigrateWithLock_Idempotent memverifikasi migrasi bisa dijalankan berulang
// tanpa error (advisory lock + goose Up idempotent). Butuh Postgres nyata via
// TEST_DATABASE_URL — di-skip bila tak ada (bukan gagal).
func TestMigrateWithLock_Idempotent(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tak di-set; lewati test migrasi")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	// DirFS di root repo; goose mencari subdir "migrations" di dalamnya
	// (sama seperti embed.FS "migrations/*.sql" di main).
	migrations := os.DirFS("../..")

	// Jalankan dua kali — kedua-duanya harus sukses (test DB sudah termigrasi
	// atau belum, keduanya OK).
	if err := MigrateWithLock(ctx, pool, migrations); err != nil {
		t.Fatalf("migrasi pertama: %v", err)
	}
	if err := MigrateWithLock(ctx, pool, migrations); err != nil {
		t.Fatalf("migrasi kedua (idempoten) gagal: %v", err)
	}

	// Sanity: tabel inti hasil migrasi ada.
	var exists bool
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name='users')`).Scan(&exists)
	if err != nil || !exists {
		t.Errorf("tabel users harus ada setelah migrasi (err=%v exists=%v)", err, exists)
	}
}
