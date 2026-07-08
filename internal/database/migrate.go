// Package database mengurus koneksi pgxpool & migrasi goose.
package database

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// advisoryLockID adalah angka arbitrer tetap untuk mengunci migrasi.
// Instance lain yang start bersamaan akan BLOCK di sini sampai lock lepas —
// mencegah race pada goose_db_version saat multi-instance.
const advisoryLockID = 4242

// MigrateWithLock menjalankan seluruh migrasi goose di dalam Postgres advisory
// lock, aman untuk startup multi-instance. migrations adalah fs.FS yang
// memuat direktori "migrations" (biasanya embed.FS).
func MigrateWithLock(ctx context.Context, pool *pgxpool.Pool, migrations fs.FS) error {
	// Ambil satu koneksi khusus untuk memegang lock selama migrasi.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockID); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}
	defer func() {
		// Best-effort unlock; koneksi tetap di-Release apa pun hasilnya.
		_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", advisoryLockID)
	}()

	// goose butuh *sql.DB (database/sql), bukan pgxpool. Bungkus pool via stdlib.
	// Menutup db TIDAK menutup pool (dikelola terpisah).
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
