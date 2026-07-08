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

// ShellData = konteks AppShell (panel /dev, /admin, /user). Terpisah dari
// LayoutData (landing/login).
type ShellData struct {
	Title       string
	BrandLabel  string // teks brand di sidebar (mis. "go_stater /dev")
	CurrentPath string // untuk active-state menu
	UserEmail   string
	AvatarURL   string
	CSSPath     string
	Nav         []NavItem
	QuickLinks  []NavItem // pintasan lintas-panel (sesuai role), di footer sidebar
}

// AppShell membungkus konten dengan layout dashboard: sidebar (menu + brand +
// footer user) + area konten. TANPA header — user pindah ke bawah sidebar.
// Sidebar bisa di-collapse jadi rail ikon (state persisten via sidebar.js +
// localStorage). Drawer mobile via signal Datastar; tombol buka = floating
// hamburger (karena header dihilangkan).
func AppShell(d ShellData, content ...g.Node) g.Node {
	cssPath := d.CSSPath
	if cssPath == "" {
		cssPath = "/static/app.css"
	}
	head := append(headNodes(cssPath),
		// sidebar.js SINKRON (bukan defer): set data-sidebar sebelum paint → no flicker.
		h.Script(h.Src("/static/sidebar.js")),
	)
	return c.HTML5(c.HTML5Props{
		Title:    d.Title,
		Language: "id",
		Head:     head,
		Body: []g.Node{
			h.Class("min-h-screen bg-background text-foreground"),
			data.Signals(map[string]any{"sidebarOpen": false}),

			// Backdrop mobile — inline display:none agar tak FOUC sebelum Datastar aktif.
			h.Div(
				h.Class("fixed inset-0 z-30 bg-black/50 md:hidden"),
				g.Attr("style", "display:none"),
				data.Show("$sidebarOpen"),
				data.On("click", "$sidebarOpen = false"),
			),

			// Floating hamburger — hanya mobile, hanya saat drawer tertutup.
			h.Button(
				h.Type("button"),
				h.Class("btn fixed top-4 left-4 z-20 md:hidden"),
				g.Attr("data-variant", "outline"),
				g.Attr("data-size", "sm"),
				g.Attr("aria-label", "Buka menu"),
				data.Show("!$sidebarOpen"),
				data.On("click", "$sidebarOpen = true"),
				lucide.Menu(h.Class("size-5")),
			),

			shellSidebar(d),

			// Konten. Margin kiri menyesuaikan lebar sidebar (rail saat collapsed).
			h.Div(
				h.Class("app-content flex min-h-screen flex-col"),
				h.Main(h.Class("flex-1 p-6"), g.Group(content)),
			),
		},
	})
}

// shellSidebar = panel navigasi. Kolom flex: brand+toggle (atas), menu (tengah,
// flex-1), footer user (bawah). Class `app-sidebar` dipakai CSS collapse rail.
func shellSidebar(d ShellData) g.Node {
	return h.Aside(
		h.Class("app-sidebar fixed inset-y-0 left-0 z-40 flex flex-col border-r "+
			"bg-sidebar text-sidebar-foreground -translate-x-full transition-transform md:translate-x-0"),
		data.Class("translate-x-0", "$sidebarOpen"),

		// Header sidebar: brand + tombol collapse (desktop).
		h.Div(
			h.Class("h-16 flex items-center gap-2 px-4 border-b border-sidebar-border"),
			h.Span(h.Class("app-brand flex-1 font-semibold truncate"), g.Text(d.BrandLabel)),
			h.Button(
				h.Type("button"),
				h.Class("btn hidden md:inline-flex"),
				g.Attr("data-variant", "ghost"),
				g.Attr("data-size", "sm"),
				g.Attr("data-sidebar-toggle", "true"),
				g.Attr("aria-label", "Perkecil sidebar"),
				lucide.PanelLeft(h.Class("size-5")),
			),
		),

		// Menu (tengah, mengisi ruang).
		h.Nav(
			h.Class("flex-1 px-3 py-4 flex flex-col gap-1 overflow-y-auto"),
			g.Map(d.Nav, func(it NavItem) g.Node { return navLink(it, d.CurrentPath) }),
		),

		// Pintasan lintas-panel (sesuai role) — di atas blok user.
		quickLinks(d),

		// Footer user (bawah): avatar + email + logout.
		sidebarUser(d),
	)
}

// quickLinks merender pintasan lintas-panel (mis. ke /dev, /admin) sesuai role.
// Kosong (nil) → tak render apa pun. Item aktif ditandai seperti navLink.
func quickLinks(d ShellData) g.Node {
	if len(d.QuickLinks) == 0 {
		return g.Text("")
	}
	return h.Div(
		h.Class("border-t border-sidebar-border px-3 py-2 flex flex-col gap-1"),
		g.Map(d.QuickLinks, func(it NavItem) g.Node { return navLink(it, d.CurrentPath) }),
	)
}

// navLink = satu item menu. Active bila path cocok (exact atau prefix section).
// title = tooltip (berguna saat rail collapsed, label tersembunyi).
func navLink(it NavItem, currentPath string) g.Node {
	active := it.Href == currentPath ||
		(it.Href != "/" && strings.HasPrefix(currentPath, it.Href))
	cls := "app-navlink flex items-center gap-3 rounded-md px-3 py-2 text-sm hover:bg-sidebar-accent"
	attrs := []g.Node{h.Href(it.Href), h.Class(cls), h.Title(it.Label)}
	if active {
		attrs = append(attrs, g.Attr("aria-current", "page"), g.Attr("data-active", "true"))
	}
	children := []g.Node{}
	if it.Icon != nil {
		children = append(children, it.Icon)
	}
	children = append(children, h.Span(h.Class("app-navlabel truncate"), g.Text(it.Label)))
	return h.A(append(attrs, children...)...)
}

// sidebarUser = blok identitas di bagian bawah sidebar (avatar, email, logout).
func sidebarUser(d ShellData) g.Node {
	return h.Div(
		h.Class("border-t border-sidebar-border p-3 flex flex-col gap-2"),
		h.Div(
			h.Class("flex items-center gap-2 min-w-0"),
			Avatar(d.AvatarURL, "", d.UserEmail, 32),
			h.Span(h.Class("app-navlabel text-sm truncate"), g.Text(d.UserEmail)),
		),
		h.Button(
			h.Class("btn w-full"),
			g.Attr("data-variant", "outline"),
			g.Attr("data-size", "sm"),
			g.Attr("title", "Keluar"),
			data.On("click", "@post('/logout')"),
			lucide.LogOut(h.Class("size-4")),
			h.Span(h.Class("app-navlabel"), g.Text("Keluar")),
		),
	)
}
