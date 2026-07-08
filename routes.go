package main

import (
	"net/http"

	"go_stater/internal/handler"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// registerRoutes mendaftarkan SEMUA route — single source of truth (§4.1).
func registerRoutes(r chi.Router, h *handler.Handler, staticFS http.Handler) {
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer) // SPIKE: middleware bawaan chi; diganti mw.Recover custom di Fase 1
	r.Use(middleware.RealIP)

	// Health
	r.Get("/healthz", h.Liveness)
	r.Get("/readyz", h.Readiness)

	// Root → todos (SPIKE: belum ada auth)
	r.Get("/", h.TodoList)

	// Todos
	r.Get("/todos", h.TodoList)
	r.Post("/todos", h.TodoCreate)
	r.Delete("/todos/{id}", h.TodoDelete)

	// Static (embedded)
	r.Handle("/static/*", staticFS)
}
