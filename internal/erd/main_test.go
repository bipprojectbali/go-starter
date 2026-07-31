package erd

import (
	"context"
	"fmt"
	"os"
	"testing"

	"go_starter/internal/testdb"

	"github.com/jackc/pgx/v5/pgxpool"
)

// main_test.go — schema Postgres milik paket ini sendiri (lihat internal/testdb).
//
// Paket ini terlewat saat schema-per-paket dikerjakan, dan test-nya tetap hijau
// — bukan karena benar, melainkan karena DB test kebetulan masih menyimpan
// tabel di `public` dari sebelum perubahan itu. Kelulusan palsu seperti itu
// baru ketahuan saat template di-clone ke database yang benar-benar baru:
// introspeksi tak menemukan apa-apa, dan gagalnya menunjuk ERD padahal
// sebabnya migrasi tak pernah dijalankan di sana.

var pkgPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()
	pool, err := testdb.Pool(ctx, "erd")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testdb:", err)
		os.Exit(1)
	}
	pkgPool = pool

	code := m.Run()

	if pool != nil {
		pool.Close()
		testdb.Drop(ctx, "erd")
	}
	os.Exit(code)
}
