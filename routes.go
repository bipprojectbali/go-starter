package main

import (
	"net/http"

	"go_stater/internal/handler"
	"go_stater/internal/mw"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// registerRoutes mendaftarkan SEMUA route — single source of truth (§4.1).
func registerRoutes(r chi.Router, h *handler.Handler, staticFS http.Handler) {
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

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
