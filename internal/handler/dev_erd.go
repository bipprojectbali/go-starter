package handler

import (
	"net/http"

	"go_stater/internal/erd"
	"go_stater/internal/ui/pages/dev"
)

// DevERD — GET /dev/erd. Memvisualisasikan relasi tabel (ERD) dari katalog
// Postgres live via Mermaid. Dev-only (didaftarkan hanya saat devMode).
func (h *Handler) DevERD(w http.ResponseWriter, r *http.Request) {
	schema, err := erd.Introspect(r.Context(), h.Pool)
	if err != nil {
		h.Log.Error("dev erd: introspect", "err", err)
		http.Error(w, "gagal membaca skema", http.StatusInternalServerError)
		return
	}
	h.renderShell(w, r, "Database ERD", "go_stater /dev", "/dev/erd", devNav(),
		dev.ERDPage(schema.Mermaid(), len(schema.Tables)))
}
