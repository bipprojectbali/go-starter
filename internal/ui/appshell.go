package ui

import (
	"strings"

	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	c "maragu.dev/gomponents/components"
	h "maragu.dev/gomponents/html"
)

// NavItem = satu entri menu sidebar.
type NavItem struct {
	Label string
	Href  string
	Icon  g.Node // ikon lucide (mis. lucide.Users(...))
}

// ShellData = konteks AppShell (panel /dev). Terpisah dari LayoutData (landing).
type ShellData struct {
	Title       string
	BrandLabel  string // teks brand di sidebar (mis. "go_stater /dev")
	CurrentPath string // untuk active-state menu
	UserEmail   string
	AvatarURL   string
	CSSPath     string
	Nav         []NavItem
}

// AppShell membungkus konten dengan layout dashboard: sidebar kiri (menu) +
// header + area konten. Dibangun sendiri (Tailwind + Datastar) — BUKAN class
// .sidebar Basecoat yang butuh JS eksternal. Drawer mobile via signal Datastar;
// desktop sidebar terkunci (md:translate-x-0).
func AppShell(d ShellData, content ...g.Node) g.Node {
	cssPath := d.CSSPath
	if cssPath == "" {
		cssPath = "/static/app.css"
	}
	return c.HTML5(c.HTML5Props{
		Title:    d.Title,
		Language: "id",
		Head:     headNodes(cssPath),
		Body: []g.Node{
			h.Class("min-h-screen bg-background text-foreground"),
			// Signal drawer mobile (default tertutup).
			data.Signals(map[string]any{"sidebarOpen": false}),

			// Backdrop mobile — inline display:none agar tak FOUC sebelum Datastar aktif.
			h.Div(
				h.Class("fixed inset-0 z-30 bg-black/50 md:hidden"),
				g.Attr("style", "display:none"),
				data.Show("$sidebarOpen"),
				data.On("click", "$sidebarOpen = false"),
			),

			shellSidebar(d),

			// Kolom kanan: header + main. md:ml-64 memberi ruang sidebar desktop.
			h.Div(
				h.Class("md:ml-64 flex min-h-screen flex-col"),
				shellHeader(d),
				h.Main(h.Class("flex-1 p-6"), g.Group(content)),
			),
		},
	})
}

// shellSidebar = panel navigasi kiri. Fixed; di mobile digeser via signal.
func shellSidebar(d ShellData) g.Node {
	return h.Aside(
		h.Class("fixed inset-y-0 left-0 z-40 w-64 border-r bg-sidebar text-sidebar-foreground "+
			"flex flex-col -translate-x-full transition-transform md:translate-x-0"),
		data.Class("translate-x-0", "$sidebarOpen"),
		// Brand.
		h.Div(
			h.Class("h-16 flex items-center px-6 border-b border-sidebar-border font-semibold"),
			g.Text(d.BrandLabel),
		),
		// Menu.
		h.Nav(
			h.Class("flex-1 px-3 py-4 flex flex-col gap-1"),
			g.Map(d.Nav, func(it NavItem) g.Node { return navLink(it, d.CurrentPath) }),
		),
	)
}

// navLink = satu item menu. Active bila path cocok (exact atau prefix section).
func navLink(it NavItem, currentPath string) g.Node {
	active := it.Href == currentPath ||
		(it.Href != "/" && strings.HasPrefix(currentPath, it.Href))
	cls := "flex items-center gap-3 rounded-md px-3 py-2 text-sm hover:bg-sidebar-accent"
	attrs := []g.Node{h.Href(it.Href), h.Class(cls)}
	if active {
		attrs = append(attrs,
			g.Attr("aria-current", "page"),
			// class tambahan tak bisa di-append ke Class yang sama; pakai atribut kedua.
			g.Attr("data-active", "true"),
		)
	}
	children := []g.Node{}
	if it.Icon != nil {
		children = append(children, it.Icon)
	}
	children = append(children, h.Span(g.Text(it.Label)))
	return h.A(append(attrs, children...)...)
}

// shellHeader = bar atas: hamburger (mobile) + spacer + avatar/email + logout.
func shellHeader(d ShellData) g.Node {
	return h.Header(
		h.Class("h-16 border-b flex items-center gap-4 px-6"),
		// Hamburger toggle (mobile).
		h.Button(
			h.Type("button"),
			h.Class("btn md:hidden"),
			g.Attr("data-variant", "ghost"),
			g.Attr("data-size", "sm"),
			g.Attr("aria-label", "Menu"),
			data.On("click", "$sidebarOpen = !$sidebarOpen"),
			lucide.Menu(h.Class("size-5")),
		),
		h.Div(h.Class("flex-1")), // spacer
		Avatar(d.AvatarURL, "", d.UserEmail, 32),
		h.Span(h.Class("text-sm hidden sm:inline"), g.Text(d.UserEmail)),
		h.Button(
			h.Class("btn"),
			g.Attr("data-variant", "outline"),
			g.Attr("data-size", "sm"),
			data.On("click", "@post('/logout')"),
			g.Text("Keluar"),
		),
	)
}
