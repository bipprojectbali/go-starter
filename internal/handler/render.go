package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"go_stater/internal/ui"

	"github.com/starfederation/datastar-go/datastar"
	g "maragu.dev/gomponents"
)

// renderPage mengirim halaman penuh (navigasi biasa, §4.4 jalur 1).
// Error render TIDAK ditelan — di-log.
func renderPage(w http.ResponseWriter, r *http.Request, log *slog.Logger, title string, body g.Node) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ui.Layout(title, body).Render(w); err != nil {
		log.Error("render page", "path", r.URL.Path, "err", err)
	}
}

// patch mengirim satu atau lebih fragment gomponents ke browser via Datastar SSE
// (§4.4 jalur 2). Elemen di-morph berdasarkan atribut id-nya (mode default outer).
func patch(w http.ResponseWriter, r *http.Request, log *slog.Logger, nodes ...g.Node) {
	sse := datastar.NewSSE(w, r) // set header SSE + flush otomatis; TIDAK return error
	for _, n := range nodes {
		var sb strings.Builder
		if err := n.Render(&sb); err != nil {
			log.Error("patch render", "path", r.URL.Path, "err", err)
			return
		}
		if err := sse.PatchElements(sb.String()); err != nil {
			log.Error("patch send", "err", err) // klien putus, dsb.
			return
		}
	}
}
