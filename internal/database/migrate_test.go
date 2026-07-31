package database_test

import (
	"context"
	"os"
	"testing"

	"go_starter/internal/database"
)

// TestMigrateWithLock_Idempotent memverifikasi migrasi bisa dijalankan berulang
// tanpa error (advisory lock + goose Up idempotent). Butuh Postgres nyata via
// TEST_DATABASE_URL — di-skip bila tak ada (bukan gagal).
func TestMigrateWithLock_Idempotent(t *testing.T) {
	if pkgPool == nil {
		t.Skip("TEST_DATABASE_URL tak di-set; lewati test migrasi")
	}
	// Pool ber-schema milik paket ini — TestMain sudah menjalankan migrasi sekali
	// di sana, jadi test ini menguji pengulangannya (yang memang intinya).
	pool := pkgPool
	ctx := context.Background()

	// DirFS di root repo; goose mencari subdir "migrations" di dalamnya
	// (sama seperti embed.FS "migrations/*.sql" di main).
	migrations := os.DirFS("../..")

	// Jalankan dua kali — kedua-duanya harus sukses (test DB sudah termigrasi
	// atau belum, keduanya OK).
	if err := database.MigrateWithLock(ctx, pool, migrations); err != nil {
		t.Fatalf("migrasi pertama: %v", err)
	}
	if err := database.MigrateWithLock(ctx, pool, migrations); err != nil {
		t.Fatalf("migrasi kedua (idempoten) gagal: %v", err)
	}

	// Sanity: tabel inti hasil migrasi ada DI SCHEMA INI. Tanpa penyaring
	// table_schema, query ini akan menemukan `users` milik schema paket lain dan
	// lulus walau migrasi di sini gagal total.
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables
		                 WHERE table_name='users' AND table_schema = current_schema())`).Scan(&exists)
	if err != nil || !exists {
		t.Errorf("tabel users harus ada setelah migrasi (err=%v exists=%v)", err, exists)
	}
}
