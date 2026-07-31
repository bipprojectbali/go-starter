package database_test

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
// Paket ini menguji MIGRASI, jadi isolasinya paling penting: menjalankan goose
// di schema bersama akan mengubah `goose_db_version` yang dipakai paket lain,
// dan riwayat migrasi yang bergeser di tengah run adalah kegagalan yang mustahil
// dibaca. Dengan schema sendiri, tabel versinya pun milik sendiri.

var pkgPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()
	pool, err := testdb.Pool(ctx, "database")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testdb:", err)
		os.Exit(1)
	}
	pkgPool = pool

	code := m.Run()

	if pool != nil {
		pool.Close()
		testdb.Drop(ctx, "database")
	}
	os.Exit(code)
}
