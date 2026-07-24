// Package handler berisi HTTP handler, satu file per fitur.
package handler

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler menampung dependency bersama seluruh handler. TIDAK ada *db.Queries
// terikat-pool di sini — akses DB SELALU lewat h.q(ctx) (Queries ber-tenant dari
// Scope middleware) atau db.WithTenant/WithSuper (jalur pre-identity). "Lupa
// scope" = compile error / panic, bukan kebocoran RLS senyap (§scope.go).
type Handler struct {
	Pool *pgxpool.Pool
	Log  *slog.Logger
}

// New membuat Handler dengan dependency ter-inject.
func New(pool *pgxpool.Pool, log *slog.Logger) *Handler {
	return &Handler{
		Pool: pool,
		Log:  log,
	}
}

// pageSize adalah batas default item per halaman (keyset pagination).
// Konstanta bernama, bukan angka telanjang (§rule 15).
const pageSize = 20
