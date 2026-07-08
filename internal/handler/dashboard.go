package handler

import (
	"net/http"

	"go_stater/internal/ui/pages/panel"
)

// AdminHome — GET /admin. Dashboard admin (AppShell). Terproteksi
// RequireEnforce("admin:home","read") di routes.
func (h *Handler) AdminHome(w http.ResponseWriter, r *http.Request) {
	h.renderShell(w, r, "Dashboard", "go_stater /admin", "/admin", adminNav,
		panel.Placeholder("Dashboard Admin", "Panel administrasi. Menu & fitur admin akan ditambahkan di sini."))
}

// UserHome — GET /user. Beranda user (AppShell). Terproteksi
// RequireEnforce("user:home","read") di routes.
func (h *Handler) UserHome(w http.ResponseWriter, r *http.Request) {
	h.renderShell(w, r, "Beranda", "go_stater", "/user", userNav,
		panel.Placeholder("Beranda", "Selamat datang. Buka Todos dari menu untuk mulai."))
}
