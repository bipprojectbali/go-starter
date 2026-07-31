package handler

import (
	"context"
	"fmt"
	"os"
	"testing"

	"go_starter/internal/testdb"

	"github.com/jackc/pgx/v5/pgxpool"
)

// main_test.go — schema Postgres milik paket ini sendiri.
//
// Sebelumnya setiap setupTest men-TRUNCATE tabel global, sehingga menjalankan
// `go test ./...` secara paralel membuat paket ini menghapus data paket lain di
// tengah jalan. Kini seluruh tabelnya hidup di schema terpisah (lihat
// internal/testdb), jadi TRUNCATE di sini hanya menyentuh milik sendiri dan
// paralel aman kembali.

// pkgPool = pool ber-schema untuk paket ini, dibagi seluruh test.
//
// Satu pool untuk semua test, bukan satu per test: membuat schema + menjalankan
// migrasi butuh ~1 detik, dan mengulanginya ratusan kali menukar satu masalah
// kecepatan dengan yang lebih besar.
var pkgPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()
	pool, err := testdb.Pool(ctx, "handler")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testdb:", err)
		os.Exit(1)
	}
	pkgPool = pool

	code := m.Run()

	if pool != nil {
		pool.Close()
		testdb.Drop(ctx, "handler")
	}
	os.Exit(code)
}
