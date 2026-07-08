package main

import (
	"log/slog"
	"net/http"

	"go_stater/internal/handler"
	"go_stater/internal/mw"

	"github.com/go-chi/chi/v5"
)

// registerRoutes mendaftarkan SEMUA route — single source of truth (§4.1).
func registerRoutes(r chi.Router, h *handler.Handler, staticFS http.Handler, log *slog.Logger) {
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

	// Publik: halaman & aksi auth
	r.Get("/login", h.LoginPage)
	r.Post("/login", h.Login)
	r.Get("/register", h.RegisterPage)
	r.Post("/register", h.Register)
	r.Post("/logout", h.Logout)

	// Protected: butuh session user
	r.Group(func(r chi.Router) {
		r.Use(mw.RequireAuth)
		r.Get("/", h.TodoList)
		r.Get("/todos", h.TodoList)
		r.Post("/todos", h.TodoCreate)
		r.Delete("/todos/{id}", h.TodoDelete)
	})
}
