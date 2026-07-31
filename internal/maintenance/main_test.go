package maintenance

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
// Paket inilah yang membongkar masalahnya: pemeliharaan menghapus baris
// KEDALUWARSA di seluruh tabel, jadi test-nya tak bisa sekadar "menandai
// barisnya sendiri" — purge tak peduli tanda. Dengan schema terpisah, yang
// dihapusnya memang hanya miliknya.

var pkgPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()
	pool, err := testdb.Pool(ctx, "maintenance")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testdb:", err)
		os.Exit(1)
	}
	pkgPool = pool

	code := m.Run()

	if pool != nil {
		pool.Close()
		testdb.Drop(ctx, "maintenance")
	}
	os.Exit(code)
}
