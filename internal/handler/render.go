package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"go_stater/internal/session"
	"go_stater/internal/ui"

	"github.com/starfederation/datastar-go/datastar"
	g "maragu.dev/gomponents"
)

// cssPath adalah path app.css dengan cache-bust hash, di-set saat startup.
var cssPath = "/static/app.css"

// SetCSSPath menetapkan path CSS ber-hash (dipanggil dari main saat startup).
func SetCSSPath(p string) { cssPath = p }

// devMode menandai environment non-production. Menentukan apakah form login
// password ditampilkan (password auth = dev-only; produksi hanya Google).
var devMode bool

// SetDevMode menetapkan flag dev (dipanggil dari main: !cfg.IsProduction()).
func SetDevMode(v bool) { devMode = v }

// renderPage mengirim halaman penuh (navigasi biasa, §4.4 jalur 1).
// Judul + email user (untuk nav) diambil dari konteks. Error di-log, tak ditelan.
func (h *Handler) renderPage(w http.ResponseWriter, r *http.Request, title string, body g.Node) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d := ui.LayoutData{
		Title:     title,
		UserEmail: h.currentUserEmail(r.Context()),
		CSSPath:   cssPath,
	}
	if err := ui.Layout(d, body).Render(w); err != nil {
		h.Log.Error("render page", "path", r.URL.Path, "err", err)
	}
}

// currentUserEmail mengambil email user login untuk nav; "" bila belum login
// atau gagal (nav akan tersembunyi).
func (h *Handler) currentUserEmail(ctx context.Context) string {
	uid := session.UserID(ctx)
	if uid == 0 {
		return ""
	}
	u, err := h.DB.GetUser(ctx, uid)
	if err != nil {
		return ""
	}
	return u.Email
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
