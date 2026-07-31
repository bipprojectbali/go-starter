package db

import (
	"context"
	"fmt"
	"os"
	"testing"

	"go_starter/internal/testdb"

	"github.com/jackc/pgx/v5/pgxpool"
)

// main_test.go — schema Postgres milik paket ini sendiri (lihat internal/testdb),
// agar `go test ./...` bisa paralel tanpa paket saling menghapus data.

var pkgPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()
	pool, err := testdb.Pool(ctx, "db")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testdb:", err)
		os.Exit(1)
	}
	pkgPool = pool

	code := m.Run()

	if pool != nil {
		pool.Close()
		testdb.Drop(ctx, "db")
	}
	os.Exit(code)
}
