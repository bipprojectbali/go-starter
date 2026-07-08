// Package handler berisi HTTP handler, satu file per fitur.
package handler

import (
	"log/slog"

	"go_stater/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler menampung dependency bersama seluruh handler.
type Handler struct {
	DB   *db.Queries
	Pool *pgxpool.Pool
	Log  *slog.Logger
}

// New membuat Handler dengan dependency ter-inject.
func New(pool *pgxpool.Pool, log *slog.Logger) *Handler {
	return &Handler{
		DB:   db.New(pool),
		Pool: pool,
		Log:  log,
	}
}

// pageSize adalah batas default item per halaman (keyset pagination).
// Konstanta bernama, bukan angka telanjang (§rule 15).
const pageSize = 20

// spikeUserID — SPIKE ONLY: auth belum diimplementasi (Fase 4). Hardcode ke
// user seed id=1. Di produksi diganti session.UserID(ctx). JANGAN merge ke main.
const spikeUserID int64 = 1
