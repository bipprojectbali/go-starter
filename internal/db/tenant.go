// tenant.go — helper RLS ditulis-tangan (BUKAN generated sqlc; sqlc tak menyentuh
// file ini). Menyediakan dua jalur eksekusi: WithTenant (scoped ke satu tenant) &
// WithSuper (bypass platform). Keduanya membungkus SATU transaksi dengan GUC
// transaction-local agar RLS Postgres (policy tenant_isolation) menggerakkan
// isolasi di level DB — "lupa WHERE tenant_id" tetap 0 baris, bukan kebocoran.
package db

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GUC transaction-local yang dibaca policy RLS tenant_isolation (migrasi 00007).
const (
	gucTenantID = "app.tenant_id" // scope: hanya baris tenant ini yang terlihat
	gucIsSuper  = "app.is_super"  // "on" = bypass RLS (jalur platform)
)

// WithTenant menjalankan fn dalam SATU transaksi yang di-scope ke tenantID via RLS.
// set_config(...,true) = TRANSACTION-LOCAL → GUC tak bocor ke peminjam pool
// berikutnya (gotcha pool-leak — kebocoran tenant #1 paling umum). fn error / panic
// → rollback; fn nil → commit.
func WithTenant(ctx context.Context, pool *pgxpool.Pool, tenantID int64, fn func(*Queries) error) error {
	return withTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config($1, $2, true)", gucTenantID, strconv.FormatInt(tenantID, 10)); err != nil {
			return err
		}
		return fn(New(tx))
	})
}

// WithSuper menjalankan fn di jalur PLATFORM (bypass RLS): set app.is_super='on'.
// HANYA dari jalur terverifikasi — auth pre-identity (tenant belum diketahui), boot
// reconcile, super_admin(env)/staff terverifikasi. JANGAN PERNAH dipicu role DB
// (anti privilege-escalation): keputusan bypass diambil di Go, bukan dari data.
func WithSuper(ctx context.Context, pool *pgxpool.Pool, fn func(*Queries) error) error {
	return withTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config($1, 'on', true)", gucIsSuper); err != nil {
			return err
		}
		return fn(New(tx))
	})
}

// withTx: buka tx, jalankan fn, commit bila nil / rollback bila error. defer
// Rollback = no-op setelah Commit sukses (pola pgx idiomatik).
func withTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck — no-op bila sudah Commit
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
