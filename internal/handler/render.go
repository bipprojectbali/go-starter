package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go_starter/internal/authz"
	"go_starter/internal/session"
	"go_starter/internal/ui"

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

// appTZ = zona waktu untuk agregasi tampilan panel logs (tampilan jam lokal).
// Default UTC bila belum di-set; di-inject dari main via SetAppTimezone.
var appTZ = time.UTC

// SetAppTimezone menetapkan zona waktu aplikasi (dipanggil dari main saat startup).
func SetAppTimezone(loc *time.Location) {
	if loc != nil {
		appTZ = loc
	}
}

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

// Menu sidebar per panel.
var (
	adminNav = []ui.NavItem{
		{Label: "Dashboard", Href: "/admin", Icon: lucide.LayoutDashboard(html.Class("size-4"))},
		{Label: "Workspace", Href: "/admin/workspace", Icon: lucide.Building2(html.Class("size-4"))},
	}
	userNav = []ui.NavItem{
		{Label: "Beranda", Href: "/user", Icon: lucide.House(html.Class("size-4"))},
	}
)

// devNav membangun menu panel /dev. "File Health" hanya di dev (route-nya tak
// terdaftar di produksi — source .go tak ada di single-binary).
func devNav() []ui.NavItem {
	items := []ui.NavItem{
		{Label: "Users", Href: "/dev/users", Icon: lucide.Users(html.Class("size-4"))},
		{Label: "User Logs", Href: "/dev/logs", Icon: lucide.ChartColumn(html.Class("size-4"))},
	}
	if devMode {
		items = append(items,
			ui.NavItem{Label: "File Health", Href: "/dev/health", Icon: lucide.Activity(html.Class("size-4"))},
			ui.NavItem{Label: "Database ERD", Href: "/dev/erd", Icon: lucide.Database(html.Class("size-4"))},
		)
	}
	return items
}

// quickLinksFor membangun pintasan lintas-panel sesuai IZIN user (Casbin,
// di-precompute di sini — bukan dari dalam gomponents). Pintasan /dev menuju
// /dev/users (route "/dev" telanjang tak ada). Hanya muncul untuk yang berhak.
func quickLinksFor(ctx context.Context) []ui.NavItem {
	var links []ui.NavItem
	if authz.Can(ctx, "dev:users", "read") {
		links = append(links, ui.NavItem{
			Label: "Developer", Href: "/dev/users", Icon: lucide.Terminal(html.Class("size-4")),
		})
	}
	if authz.Can(ctx, "admin:home", "read") {
		links = append(links, ui.NavItem{
			Label: "Admin", Href: "/admin", Icon: lucide.Shield(html.Class("size-4")),
		})
	}
	if authz.Can(ctx, "user:home", "read") {
		links = append(links, ui.NavItem{
			Label: "User", Href: "/user", Icon: lucide.House(html.Class("size-4")),
		})
	}
	return links
}

// renderShell mengirim halaman dengan AppShell (sidebar). brand = label brand
// di sidebar, currentPath untuk active-state, nav = menu panel.
func (h *Handler) renderShell(w http.ResponseWriter, r *http.Request, title, brand, currentPath string, nav []ui.NavItem, body g.Node) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d := ui.ShellData{
		Title:         title,
		BrandLabel:    brand,
		WorkspaceName: session.TenantName(r.Context()), // brand utama; "" utk platform → fallback BrandLabel
		CurrentPath:   currentPath,
		UserEmail:     session.Email(r.Context()),
		AvatarURL:     session.AvatarURL(r.Context()),
		CSSPath:       cssPath,
		Nav:           nav,
		QuickLinks:    quickLinksFor(r.Context()),
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
