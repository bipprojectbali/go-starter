package main

import (
	"log/slog"
	"net/http"

	"go_stater/internal/handler"
	"go_stater/internal/mw"

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

	// Publik: landing page (TIDAK redirect ke /login). RefreshIdentity agar
	// redirect per-role pakai role SEGAR dari DB (self-heal session lama).
	// TrackPresence: rekam kehadiran bila user login (no-op utk anonim).
	r.With(h.RefreshIdentity, h.TrackPresence).Get("/", h.Home)

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
		r.Use(h.RefreshIdentity)
		r.Use(h.TrackPresence)
		r.Use(mw.RequireEnforce("admin:home", "read"))
		r.Get("/", h.AdminHome)
	})

	// Beranda /user — semua user login (admin/super mewarisi user:home).
	r.Route("/user", func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(h.RefreshIdentity)
		r.Use(h.TrackPresence)
		r.Use(mw.RequireEnforce("user:home", "read"))
		r.Get("/", h.UserHome)
	})
}
