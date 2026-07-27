package main

import (
	"log/slog"
	"net/http"

	"go_starter/internal/handler"
	"go_starter/internal/mw"

	"github.com/go-chi/chi/v5"
)

// registerRoutes mendaftarkan SEMUA route — single source of truth (§4.1).
// devMode (=!production) menentukan apakah auth password diaktifkan; di produksi
// hanya Google yang jadi jalur login.
func registerRoutes(r chi.Router, h *handler.Handler, staticFS http.Handler, log *slog.Logger, devMode bool) {
	// Urutan middleware penting: request-id dulu (dipakai log/recover),
	// lalu recover (tangkap panic downstream), log, security headers.
	r.Use(mw.RequestID)
	r.Use(mw.Recover(log))
	r.Use(mw.RequestLog(log))
	r.Use(mw.SecurityHeaders)

	// Health (tanpa auth)
	r.Get("/healthz", h.Liveness)
	r.Get("/readyz", h.Readiness)

	// Static (embedded, tanpa auth)
	r.Handle("/static/*", staticFS)

	// Browser tetap auto-minta /favicon.ico di root (bookmark, tab lama) walau
	// <head> menunjuk favicon.svg — redirect permanen agar tak jadi 404 di log.
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/favicon.svg", http.StatusMovedPermanently)
	})

	// Publik: landing page (TIDAK redirect ke /login). Scope buka tx ber-tenant
	// bila login (no-op anonim) — RefreshIdentity/TrackPresence butuh h.q(ctx).
	// RefreshIdentity agar redirect per-role pakai role SEGAR dari DB (self-heal
	// session lama). TrackPresence: rekam kehadiran bila login (no-op anonim).
	r.With(h.Scope, h.RefreshIdentity, h.TrackPresence).Get("/", h.Home)

	// Login Google — SELALU aktif (jalur login utama di produksi). Path pakai
	// prefix /api/auth/ agar exact-match dengan redirect URI di Google Console.
	r.Get("/api/auth/google", h.GoogleLogin)
	r.Get("/api/auth/callback/google", h.GoogleCallback)

	// GET /login selalu ada (target redirect RequireAuth); rendernya adaptif
	// (form password hanya muncul di dev — lihat handler.SetDevMode).
	r.Get("/login", h.LoginPage)
	r.Post("/logout", h.Logout)

	// Auth password — DEV-ONLY. Di produksi permukaan serang ini tidak ada;
	// hanya untuk mempermudah agen/dev masuk ke runtime.
	if devMode {
		r.Post("/login", h.Login)
		r.Get("/register", h.RegisterPage)
		r.Post("/register", h.Register)
	}

	// Semua route terproteksi memakai urutan middleware sama:
	// RequireAuth (authn) → RefreshIdentity (role/status SEGAR dari DB, self-heal
	// session lama + enforcement real-time) → RequireEnforce (authz per resource).

	// Panel /dev — owner/developer. Hanya super_admin/root lolos "dev:users".
	r.Route("/dev", func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(h.Scope) // buka tx ber-tenant (RLS) SEBELUM RefreshIdentity butuh h.q
		r.Use(h.RefreshIdentity)
		r.Use(h.TrackPresence)
		r.Use(mw.RequireEnforce("dev:users", "read"))
		// /dev telanjang → arahkan ke halaman default panel (/dev/users). Tanpa
		// ini, /dev 404 (dan HomePath super_admin menuju /dev).
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/dev/users", http.StatusSeeOther)
		})
		r.Get("/users", h.DevUsersList)
		r.Post("/users/{id}/role", h.DevUserSetRole)
		r.Post("/users/{id}/status", h.DevUserSetStatus)
		r.Post("/users/{id}/delete", h.DevUserDelete)

		// Panel aktivitas user — TERSEDIA di produksi (data-driven, super_admin
		// butuh pantau aktivitas nyata). Beda dari /health & /erd yang dev-only.
		r.Get("/logs", h.DevLogs)

		// File Health — DEV-ONLY. Butuh source .go di disk (tak ada di
		// single-binary produksi). Tak didaftarkan di prod → menu pun tak muncul.
		if devMode {
			r.Get("/health", h.DevHealth)
			r.Get("/erd", h.DevERD)
		}
	})

	// Panel /admin — admin+ (super_admin mewarisi). Konten menyusul.
	r.Route("/admin", func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(h.Scope) // buka tx ber-tenant (RLS) SEBELUM RefreshIdentity butuh h.q
		r.Use(h.RefreshIdentity)
		r.Use(h.TrackPresence)
		r.Use(mw.RequireEnforce("admin:home", "read"))
		r.Get("/", h.AdminHome)
		// Pengaturan workspace: semua penghuni /admin boleh LIHAT (admin:home).
		// Ganti nama = owner/platform saja — di-guard di handler (canEditWorkspace),
		// bukan route, agar admin tetap bisa membuka halamannya (read-only).
		r.Get("/workspace", h.WorkspaceSettings)
		r.Post("/workspace", h.WorkspaceUpdate)

		// Anggota workspace (model membership). Lihat = admin+; ubah/keluarkan/
		// undang = owner/admin (di-guard handler via canManageMembers).
		r.Get("/members", h.MembersPage)
		r.Post("/members/{id}/role", h.MemberSetRole)
		r.Post("/members/{id}/remove", h.MemberRemove)
		r.Post("/members/invite", h.InviteCreate)
		r.Post("/members/invite/{id}/delete", h.InviteDelete)
	})

	// Workspace: pindah & buat baru. Butuh login TAPI di luar grup /admin —
	// /workspace/new adalah tujuan Scope saat user belum punya workspace sama
	// sekali (jadi TIDAK boleh butuh scope tenant yang belum ada).
	r.Group(func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(h.Scope)
		r.Use(h.RefreshIdentity)
		r.Post("/workspace/switch", h.WorkspaceSwitch)
	})
	r.Group(func(r chi.Router) {
		r.Use(mw.RequireAuth)
		// TANPA Scope: user tanpa workspace tak punya tenant untuk di-scope.
		// Handler pakai db.WithSuper langsung (membership belum ada).
		r.Get("/workspace/new", h.WorkspaceNewPage)
		r.Post("/workspace/new", h.WorkspaceCreate)
	})

	// Undangan — PUBLIK (penerima belum tentu punya akun). Tanpa Scope: penerima
	// belum jadi anggota workspace mana pun saat membuka tautan.
	r.Get("/invite/{token}", h.InvitePage)
	r.Post("/invite/{token}/accept", h.InviteAccept)

	// Notifikasi — LINTAS-PANEL (bukan milik /admin maupun /user): undangan bisa
	// datang dari workspace yang belum jadi milik user. Scope tetap dipasang agar
	// shell (switcher workspace) punya Queries ber-scope; data notifikasinya
	// sendiri dibaca via db.WithSuper karena lintas-workspace.
	r.Route("/notifications", func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(h.Scope)
		r.Use(h.RefreshIdentity)
		r.Use(h.TrackPresence)
		r.Use(mw.RequireEnforce("notif:home", "read"))
		r.Get("/", h.NotificationsPage)
		r.Post("/invite/{token}/accept", h.NotificationAccept)
		r.Post("/invite/{token}/decline", h.NotificationDecline)
	})

	// Beranda /user — semua user login (admin/super mewarisi user:home).
	r.Route("/user", func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(h.Scope) // buka tx ber-tenant (RLS) SEBELUM RefreshIdentity butuh h.q
		r.Use(h.RefreshIdentity)
		r.Use(h.TrackPresence)
		r.Use(mw.RequireEnforce("user:home", "read"))
		r.Get("/", h.UserHome)
	})
}
