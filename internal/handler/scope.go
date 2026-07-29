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
		// Di mode single ini selalu terisi (slug tenant tunggal) — jadi seluruh
		// cabang di bawah berjalan sama persis seperti mode multi, tanpa satu pun
		// `if mode` tambahan (0006 §1: satu jalur kode, bukan dua).
		slug := slugFromRequest(r)

		// Platform (super_admin/staff) lintas-tenant → bypass RLS, tak perlu membership.
		if isPlatformRole(session.Role(ctx)) {
			// Tapi slug di path tetap WAJIB diikuti: handler membaca workspace dari
			// session.TenantID, jadi tanpa ini platform yang membuka /w/acme/members
			// akan melihat anggota workspace LAIN (yang kebetulan aktif di session)
			// di bawah URL yang menjanjikan acme — salah data secara senyap.
			if slug != "" && !h.adoptTenantBySlug(ctx, slug) {
				http.NotFound(w, r)
				return
			}
			// SENGAJA TANPA gateLifecycle (0005): platform tetap boleh masuk
			// workspace yang ditangguhkan/diarsipkan/terhapus. Merekalah yang
			// menangguhkan — menghalangi mereka membuat suspensi mustahil
			// diselidiki, dan restore mustahil diverifikasi sebelum ditekan.
			// Ini keputusan, bukan kelalaian; dikunci TestPlatformTembusSuspensi.
			if err := db.WithSuper(ctx, h.Pool, run); err != nil {
				h.Log.Error("scope: tx super", "err", err)
			}
			return
		}

		// Tenant user: VALIDASI keanggotaan sebelum membuka tx ber-tenant. Baik slug
		// di path MAUPUN tenantID di session user-controlled → tanpa cek ini,
		// workspace sembarang bisa dipaksa.
		//
		// Slug di path MENANG atas session (0004): URL adalah state yang lengkap,
		// jadi dua tab bisa membuka workspace berbeda tanpa saling merusak.
		if slug != "" {
			t, ok := h.resolveTenantBySlug(ctx, uid, slug)
			if !ok {
				// 404, BUKAN 403 dan BUKAN redirect: 403 mengonfirmasi workspace itu
				// ada (kebocoran keberadaan), redirect ke workspace lain menampilkan
				// data yang salah secara senyap — penyakit yang justru diobati 0004.
				http.NotFound(w, r)
				return
			}
			// Gerbang siklus hidup (0005) — SETELAH keanggotaan terbukti. Urutannya
			// penting: yang dilindungi 0004 adalah keberadaan workspace dari ORANG
			// LUAR, bukan alasan dari orang dalam.
			if !h.gateLifecycle(w, r, t) {
				return
			}
			ctx = withTenantStatus(ctx, t.Status)
			run = func(q *db.Queries) error {
				next.ServeHTTP(w, r.WithContext(withQueries(ctx, q)))
				return nil
			}
			if err := db.WithTenant(ctx, h.Pool, t.ID, run); err != nil {
				h.Log.Error("scope: tx tenant", "err", err)
			}
			return
		}

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

// adoptTenantBySlug menjadikan workspace ber-slug tsb sebagai konteks aktif TANPA
// memeriksa keanggotaan — khusus role platform, yang memang lintas-workspace
// (super_admin/staff membantu workspace mana pun). false bila slug tak ada.
//
// Terpisah dari resolveTenantBySlug justru agar bedanya kelihatan: yang ini
// SENGAJA tanpa cek membership, dan keputusan itu diambil dari ROLE — tak pernah
// dari data DB (anti privilege-escalation, sama seperti WithSuper).
func (h *Handler) adoptTenantBySlug(ctx context.Context, slug string) bool {
	t, ok := h.tenantBySlug(ctx, slug)
	if !ok {
		return false
	}
	if t.ID != session.TenantID(ctx) {
		session.SetActiveTenant(ctx, t.ID, t.Name, t.Slug)
	}
	return true
}

// tenantBySlug membaca tenant dari slug TANPA memeriksa keanggotaan. Hanya untuk
// jalur PLATFORM (lintas-workspace) — pemanggil wajib sudah memastikan rolenya
// dari session, tak pernah dari data DB (anti privilege-escalation).
func (h *Handler) tenantBySlug(ctx context.Context, slug string) (db.Tenant, bool) {
	var (
		tenant db.Tenant
		found  bool
	)
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		t, e := q.GetTenantBySlug(ctx, slug)
		if e != nil {
			return nil // slug tak ada → 404, bukan error
		}
		tenant, found = t, true
		return nil
	})
	if err != nil {
		h.Log.Error("scope: baca slug", "slug", slug, "err", err)
		return db.Tenant{}, false
	}
	return tenant, found
}

// resolveTenantBySlug menerjemahkan slug di URL ke tenant_id, HANYA bila user
// benar-benar anggotanya. false untuk slug tak dikenal MAUPUN slug milik orang
// lain — dua keadaan itu sengaja tak dibedakan agar pemanggil (404) tidak
// membocorkan workspace mana yang ada.
//
// Session ikut diperbarui agar jalur tanpa slug berikutnya (/notifications,
// landing) mengikuti workspace yang sedang dibuka — session sebagai PETUNJUK,
// bukan sumber kebenaran (0004).
//
// Dibaca lewat WithSuper: memberships & tenants sengaja tanpa RLS, dan query ini
// justru yang MENENTUKAN tenant — tak bisa bergantung GUC yang belum di-set.
// Keamanan dari filter user_id = uid sesi.
func (h *Handler) resolveTenantBySlug(ctx context.Context, uid int64, slug string) (db.Tenant, bool) {
	var (
		tenant db.Tenant
		name   string
		found  bool
	)
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		t, e := q.GetTenantBySlug(ctx, slug)
		if e != nil {
			return nil // slug tak ada — bukan error, cukup tak ditemukan
		}
		if _, e := q.GetMembership(ctx, db.GetMembershipParams{UserID: uid, TenantID: t.ID}); e != nil {
			return nil // bukan anggota — disamakan dgn tak ada (anti kebocoran keberadaan)
		}
		tenant, name, found = t, t.Name, true
		return nil
	})
	if err != nil {
		h.Log.Error("scope: resolve slug", "user_id", uid, "err", err)
		return db.Tenant{}, false
	}
	if found && tenant.ID != session.TenantID(ctx) {
		session.SetActiveTenant(ctx, tenant.ID, name, slug)
	}
	return tenant, found
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
		okSlug   string
		found    bool
	)
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		if want != 0 && session.TenantSlug(ctx) != "" {
			// Slug ikut disyaratkan: session lama (sebelum 0004) menyimpan tenantID
			// tanpa slug. Tanpa cek ini ia lolos sebagai "valid" lalu setiap wsPath
			// menghasilkan /workspace/new — self-heal lewat jalur fallback di bawah.
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
			okTenant, okName, okSlug, found = ms[0].TenantID, ms[0].Name, ms[0].Slug, true
		}
		return nil
	})
	if err != nil {
		h.Log.Error("scope: resolve tenant", "user_id", uid, "err", err)
		return 0, false
	}
	if found && okSlug != "" {
		session.SetActiveTenant(ctx, okTenant, okName, okSlug) // fallback → simpan
	}
	return okTenant, found
}

// isPlatformRole melaporkan apakah role = operator platform (bypass RLS).
// super_admin (env) & staff (platform_staff) keduanya lintas-tenant.
func isPlatformRole(role string) bool {
	return role == authz.RoleNameSuperAdmin || role == authz.RoleNameStaff
}
