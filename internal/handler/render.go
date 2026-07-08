package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"go_stater/internal/session"
	"go_stater/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"github.com/starfederation/datastar-go/datastar"
	g "maragu.dev/gomponents"
	html "maragu.dev/gomponents/html"
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

// isSuperAdminEmail memeriksa apakah email = super-admin env (root immutable).
// Di-inject dari main (config.IsSuperAdminEmail) agar handler tak import config.
var isSuperAdminEmail = func(string) bool { return false }

// SetSuperAdminChecker menyuntik fungsi cek super-admin env dari config.
func SetSuperAdminChecker(fn func(string) bool) { isSuperAdminEmail = fn }

// renderPage mengirim halaman penuh (navigasi biasa, §4.4 jalur 1). Email &
// avatar untuk nav dibaca dari SESSION (di-set saat login) — tanpa hit DB tiap
// render. Error di-log, tak ditelan.
func (h *Handler) renderPage(w http.ResponseWriter, r *http.Request, title string, body g.Node) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d := ui.LayoutData{
		Title:     title,
		UserEmail: session.Email(r.Context()),
		AvatarURL: session.AvatarURL(r.Context()),
		CSSPath:   cssPath,
	}
	if err := ui.Layout(d, body).Render(w); err != nil {
		h.Log.Error("render page", "path", r.URL.Path, "err", err)
	}
}

// devNav = menu sidebar panel /dev.
var devNav = []ui.NavItem{
	{Label: "Users", Href: "/dev/users", Icon: lucide.Users(html.Class("size-4"))},
}

// renderShell mengirim halaman dengan AppShell (sidebar) untuk panel /dev.
func (h *Handler) renderShell(w http.ResponseWriter, r *http.Request, title, currentPath string, body g.Node) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d := ui.ShellData{
		Title:       title,
		BrandLabel:  "go_stater /dev",
		CurrentPath: currentPath,
		UserEmail:   session.Email(r.Context()),
		AvatarURL:   session.AvatarURL(r.Context()),
		CSSPath:     cssPath,
		Nav:         devNav,
	}
	if err := ui.AppShell(d, body).Render(w); err != nil {
		h.Log.Error("render shell", "path", r.URL.Path, "err", err)
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
