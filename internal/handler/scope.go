package handler

import (
	"context"
	"net/http"

	"go_stater/internal/authz"
	"go_stater/internal/db"
	"go_stater/internal/session"
)

// scope.go — jantung anti-footgun multi-tenancy. Menggantikan h.DB (Queries
// terikat-pool, tanpa scope) dengan h.q(ctx) yang mengembalikan *db.Queries
// ber-tenant dari transaksi request. "Lupa scope" jadi PANIC keras di dev, bukan
// kebocoran tenant senyap. Isolasi sebenarnya ditegakkan Postgres RLS (migrasi
// 00007) — GUC transaction-local yang di-set WithTenant/WithSuper (internal/db).

type scopeCtxKey struct{}

// withQueries menaruh Queries ber-scope ke context (dipanggil middleware Scope).
func withQueries(ctx context.Context, q *db.Queries) context.Context {
	return context.WithValue(ctx, scopeCtxKey{}, q)
}

// q mengambil *db.Queries ber-tenant dari context request. PANIC bila Scope tak
// terpasang di rantai middleware — kesalahan wiring ketahuan seketika (dev),
// bukan jadi query lintas-tenant tak ter-scope. Handler SELALU pakai h.q(ctx),
// TIDAK PERNAH h.DB (yang sudah dihapus).
func (h *Handler) q(ctx context.Context) *db.Queries {
	q, ok := ctx.Value(scopeCtxKey{}).(*db.Queries)
	if !ok || q == nil {
		panic("handler.q: Scope middleware tidak terpasang — RLS tak ter-scope (bug wiring routes.go)")
	}
	return q
}

// Scope membuka SATU transaksi ber-scope per request dan menaruh Queries-nya di
// context. Platform (super_admin/staff) → WithSuper (bypass RLS); tenant user →
// WithTenant(session.TenantID). Anonim → next tanpa tx (handler terproteksi tak
// akan lolos RequireAuth). Pasang SETELAH RequireAuth, SEBELUM RefreshIdentity/
// TrackPresence (keduanya butuh h.q). Transaksi commit bila handler tak menulis
// error ke pipeline — di sini SELALU commit (handler pakai SSE/HTTP langsung,
// bukan return error); RLS + WITH CHECK yang menjaga integritas, bukan rollback.
func (h *Handler) Scope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if session.UserID(ctx) == 0 {
			next.ServeHTTP(w, r) // anonim — tak ada scope (route terproteksi butuh RequireAuth)
			return
		}
		run := func(q *db.Queries) error {
			next.ServeHTTP(w, r.WithContext(withQueries(ctx, q)))
			return nil
		}
		var err error
		if isPlatformRole(session.Role(ctx)) {
			err = db.WithSuper(ctx, h.Pool, run)
		} else {
			err = db.WithTenant(ctx, h.Pool, session.TenantID(ctx), run)
		}
		if err != nil {
			// Commit/begin gagal SETELAH response ditulis handler — tak bisa ubah
			// status. Log saja (fail-visible). Isolasi tetap terjaga (tx rollback).
			h.Log.Error("scope: tx", "err", err)
		}
	})
}

// isPlatformRole melaporkan apakah role = operator platform (bypass RLS).
// super_admin (env) & staff (platform_staff) keduanya lintas-tenant.
func isPlatformRole(role string) bool {
	return role == authz.RoleNameSuperAdmin || role == authz.RoleNameStaff
}
