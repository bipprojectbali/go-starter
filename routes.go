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

	// Publik: landing page (TIDAK redirect ke /login).
	r.Get("/", h.Home)

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

	// Protected: butuh session user
	r.Group(func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Get("/todos", h.TodoList)
		r.Post("/todos", h.TodoCreate)
		r.Delete("/todos/{id}", h.TodoDelete)
	})

	// Panel /dev — owner/developer. RequireAuth (authn) lalu RequireEnforce
	// (authz Casbin: hanya super_admin/root lolos "dev:users").
	r.Route("/dev", func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(mw.RequireEnforce("dev:users", "read"))
		r.Get("/users", h.DevUsersList)
		r.Post("/users/{id}/role", h.DevUserSetRole)
		r.Post("/users/{id}/status", h.DevUserSetStatus)
		r.Post("/users/{id}/delete", h.DevUserDelete)
	})

	// Panel /admin — admin+ (super_admin mewarisi). Konten menyusul.
	r.Route("/admin", func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(mw.RequireEnforce("admin:home", "read"))
		r.Get("/", h.AdminHome)
	})

	// Beranda /user — semua user login (admin/super mewarisi user:home).
	r.Route("/user", func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Use(mw.RequireEnforce("user:home", "read"))
		r.Get("/", h.UserHome)
	})
}
