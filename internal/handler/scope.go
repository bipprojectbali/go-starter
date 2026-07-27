package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
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
		uid := session.UserID(ctx)
		if uid == 0 {
			next.ServeHTTP(w, r) // anonim — tak ada scope (route terproteksi butuh RequireAuth)
			return
		}
		run := func(q *db.Queries) error {
			next.ServeHTTP(w, r.WithContext(withQueries(ctx, q)))
			return nil
		}
		// Platform (super_admin/staff) lintas-tenant → bypass RLS, tak perlu membership.
		if isPlatformRole(session.Role(ctx)) {
			if err := db.WithSuper(ctx, h.Pool, run); err != nil {
				h.Log.Error("scope: tx super", "err", err)
			}
			return
		}

		// Tenant user: VALIDASI keanggotaan sebelum membuka tx ber-tenant. Session
		// user-controlled → tanpa cek ini, tenantID sembarang bisa dipaksa.
		tenantID, ok := h.resolveActiveTenant(ctx, uid)
		if !ok {
			// Belum punya workspace sama sekali (mis. membership terakhir dicabut)
			// → arahkan buat workspace baru. Bukan error: keadaan sah di model ini.
			http.Redirect(w, r, "/workspace/new", http.StatusSeeOther)
			return
		}
		if err := db.WithTenant(ctx, h.Pool, tenantID, run); err != nil {
			// Commit/begin gagal SETELAH response ditulis handler — tak bisa ubah
			// status. Log saja (fail-visible). Isolasi tetap terjaga (tx rollback).
			h.Log.Error("scope: tx tenant", "err", err)
		}
	})
}

// resolveActiveTenant mengembalikan workspace aktif yang SUDAH TERVALIDASI milik
// user. Alur: pakai tenant di session bila user memang anggotanya; kalau tidak
// (session basi / dipaksa / baru login) → jatuh ke workspace pertama user dan
// simpan sebagai aktif. false bila user tak punya workspace sama sekali.
//
// Dibaca lewat WithSuper karena memberships SENGAJA tanpa RLS: query ini justru
// yang MENENTUKAN tenant — tak bisa bergantung GUC yang belum di-set (chicken-and-
// egg). Keamanan dari filter user_id = uid sesi, bukan RLS.
func (h *Handler) resolveActiveTenant(ctx context.Context, uid int64) (int64, bool) {
	want := session.TenantID(ctx)
	var (
		okTenant int64
		okName   string
		found    bool
	)
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		if want != 0 {
			if _, e := q.GetMembership(ctx, db.GetMembershipParams{UserID: uid, TenantID: want}); e == nil {
				okTenant, found = want, true
				return nil // session valid — tak perlu query lagi
			}
		}
		ms, e := q.ListMembershipsByUser(ctx, uid)
		if e != nil {
			return e
		}
		if len(ms) > 0 {
			okTenant, okName, found = ms[0].TenantID, ms[0].Name, true
		}
		return nil
	})
	if err != nil {
		h.Log.Error("scope: resolve tenant", "user_id", uid, "err", err)
		return 0, false
	}
	if found && okTenant != want {
		session.SetActiveTenant(ctx, okTenant, okName) // fallback → simpan
	}
	return okTenant, found
}

// isPlatformRole melaporkan apakah role = operator platform (bypass RLS).
// super_admin (env) & staff (platform_staff) keduanya lintas-tenant.
func isPlatformRole(role string) bool {
	return role == authz.RoleNameSuperAdmin || role == authz.RoleNameStaff
}
