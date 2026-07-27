package handler

import (
	"net/http"
	"os"

	"go_starter/internal/health"
	"go_starter/internal/ui/pages/dev"
)

// DevHealth — GET /dev/health. Memindai kesehatan file sumber .go proyek dari
// disk (cwd = root repo saat dev). Route ini HANYA didaftarkan di dev
// (lihat routes.go) karena source tak ada di build single-binary produksi.
func (h *Handler) DevHealth(w http.ResponseWriter, r *http.Request) {
	res, err := health.Scan(os.DirFS("."))
	if err != nil {
		h.Log.Error("dev health: scan", "err", err)
		http.Error(w, "gagal memindai file", http.StatusInternalServerError)
		return
	}
	h.renderShell(w, r, "File Health", "go_starter /dev", "/dev/health", devNav(),
		dev.HealthPage(res))
}
