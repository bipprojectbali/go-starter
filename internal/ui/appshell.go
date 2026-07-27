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
	Title         string
	BrandLabel    string // konteks panel di sidebar (mis. "go_starter /dev")
	WorkspaceName string // nama workspace (tenant) user — brand utama; "" utk platform tanpa konteks
	CurrentPath   string // untuk active-state menu
	UserEmail     string
	AvatarURL     string
	CSSPath       string
	Nav           []NavItem
	QuickLinks    []NavItem // pintasan lintas-panel (sesuai role), di footer sidebar
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
			// Latar dasar = base-200; sidebar & card = base-100 (permukaan
			// menonjol). Hierarki relatif ini benar otomatis di semua tema.
			h.Class("min-h-screen bg-base-200 text-base-content"),
			data.Signals(map[string]any{"sidebarOpen": false, "logoutConfirm": false}),

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
				h.Class("btn btn-outline btn-sm fixed top-4 left-4 z-20 md:hidden"),
				g.Attr("aria-label", "Buka menu"),
				data.Show("!$sidebarOpen"),
				data.On("click", "$sidebarOpen = true"),
				lucide.Menu(h.Class("size-5")),
			),

			shellSidebar(d),

			// Konten. Margin kiri menyesuaikan lebar sidebar (rail saat collapsed).
			// pt-16 di mobile memberi ruang untuk floating hamburger (top-4, fixed)
			// agar tak menimpa judul konten; md: kembali ke padding normal (sidebar
			// tetap tampil, hamburger hilang → tak perlu ruang ekstra).
			h.Div(
				h.Class("app-content flex min-h-screen flex-col"),
				h.Main(h.Class("flex-1 px-4 pb-6 pt-16 sm:px-6 md:p-6"), g.Group(content)),
			),

			// Modal konfirmasi logout (dipicu tombol Keluar).
			ConfirmModal("logoutConfirm", "Keluar?",
				"Anda akan keluar dari sesi ini.", "Keluar", "/logout"),
		},
	})
}

// shellBrand merender blok brand di header sidebar: nama workspace sebagai baris
// utama + konteks panel (BrandLabel, mis. "go_starter /admin") sebagai sub-label
// kecil. Bila WorkspaceName kosong (platform tanpa konteks tenant), tampilkan
// hanya BrandLabel (fallback perilaku lama). Kelas app-navlabel disembunyikan saat
// rail collapsed (konsisten label lain).
func shellBrand(d ShellData) g.Node {
	if d.WorkspaceName == "" {
		return h.Span(h.Class("app-brand flex-1 font-semibold truncate"), g.Text(d.BrandLabel))
	}
	return h.Div(
		h.Class("app-brand flex-1 min-w-0 flex flex-col justify-center leading-tight"),
		h.Span(h.Class("font-semibold truncate"), g.Text(d.WorkspaceName)),
		h.Span(h.Class("app-navlabel text-xs text-base-content/60 truncate"), g.Text(d.BrandLabel)),
	)
}

// shellSidebar = panel navigasi. Kolom flex: brand+toggle (atas), menu (tengah,
// flex-1), footer user (bawah). Class `app-sidebar` dipakai CSS collapse rail.
func shellSidebar(d ShellData) g.Node {
	return h.Aside(
		h.Class("app-sidebar fixed inset-y-0 left-0 z-40 flex flex-col border-r border-base-300 "+
			"bg-base-100 text-base-content -translate-x-full transition-transform md:translate-x-0"),
		// ClassOn mengutip nama class otomatis → key ber-hyphen mustahil salah
		// (gotcha #5). Bandingkan data.Class mentah yang butuh kutip manual.
		ClassOn("translate-x-0", "$sidebarOpen"),

		// Header sidebar: brand + tombol collapse (desktop). Brand = nama workspace
		// (utama) + konteks panel (sub-label kecil). Platform tanpa nama → BrandLabel.
		h.Div(
			h.Class("h-16 flex items-center gap-2 px-4 border-b border-base-300"),
			shellBrand(d),
			h.Button(
				h.Type("button"),
				h.Class("btn btn-ghost btn-sm hidden md:inline-flex"),
				g.Attr("data-sidebar-toggle", "true"),
				g.Attr("aria-label", "Perkecil sidebar"),
				lucide.PanelLeft(h.Class("size-5")),
			),
		),

		// Menu (tengah, mengisi ruang). activeHref dihitung SEKALI (longest-match)
		// agar hanya satu item menyala walau ada sub-route (mis. /admin/workspace).
		h.Nav(
			h.Class("flex-1 px-3 py-4 flex flex-col gap-1 overflow-y-auto"),
			navList(d.Nav, d.CurrentPath),
		),

		// Pintasan lintas-panel (sesuai role) — di atas blok user.
		quickLinks(d),

		// Footer user (bawah): avatar + email + logout.
		sidebarUser(d),
	)
}

// navList merender daftar item + menandai satu yang aktif (longest-match dihitung
// sekali, bukan per-item). Dipakai menu utama & quickLinks.
func navList(items []NavItem, currentPath string) g.Node {
	active := activeNavHref(items, currentPath)
	return g.Map(items, func(it NavItem) g.Node { return navLink(it, active) })
}

// quickLinks merender pintasan lintas-panel (mis. ke /dev, /admin) sesuai role.
// Kosong (nil) → tak render apa pun. Item aktif ditandai seperti navLink.
func quickLinks(d ShellData) g.Node {
	if len(d.QuickLinks) == 0 {
		return g.Text("")
	}
	return h.Div(
		h.Class("border-t border-base-300 px-3 py-2 flex flex-col gap-1"),
		navList(d.QuickLinks, d.CurrentPath),
	)
}

// activeNavHref mengembalikan href TUNGGAL yang aktif untuk currentPath dari
// daftar nav: LONGEST-MATCH menang. "/admin/workspace" mengaktifkan Workspace
// (href persis), BUKAN Dashboard (href "/admin") — dulu prefix telanjang bikin
// keduanya menyala. Match = exact ATAU sub-path pada batas segmen (href+"/").
// "" bila tak ada yang cocok.
func activeNavHref(nav []NavItem, currentPath string) string {
	best := ""
	for _, it := range nav {
		if it.Href == "/" {
			if currentPath == "/" && len(best) == 0 {
				best = "/"
			}
			continue
		}
		match := it.Href == currentPath || strings.HasPrefix(currentPath, it.Href+"/")
		if match && len(it.Href) > len(best) {
			best = it.Href
		}
	}
	return best
}

// navLink = satu item menu. active bila href-nya = activeHref (dihitung sekali di
// navList). title = tooltip (berguna saat rail collapsed, label tersembunyi).
func navLink(it NavItem, activeHref string) g.Node {
	active := it.Href == activeHref
	// Base + garis kiri transparan (border-l-2 border-transparent) SELALU ada agar
	// lebar/posisi item konsisten active vs non-active (tak bergeser 2px). Hover
	// ditambah per-state agar tak konflik dengan bg active.
	cls := "app-navlink flex items-center gap-3 rounded-md border-l-2 border-transparent px-3 py-2 text-sm"
	if active {
		// Active = SOFT: bg primary transparan tipis + garis kiri primary (aksen
		// warna brand LEMBUT, tak mencolok) — TAPI teks = base-content (bukan
		// text-primary). Alasan: sebagian tema punya primary sangat terang (cupcake
		// oklch 85%) → text-primary di latar terang kontras lemah/tak terbaca.
		// base-content dijamin kontras dgn base di SEMUA tema (token semantik daisyUI).
		// Warna active tetap terasa dari bg+border, keterbacaan tetap AA. Hover naik
		// tipis (masih transparan) — teks tak pernah hilang (beda dari bug lama:
		// hover:bg-base-200 menimpa bg solid → teks putih di atas abu terang).
		cls += " bg-primary/10 text-base-content font-medium border-primary hover:bg-primary/20"
	} else {
		// Non-active: hover netral (surface sedikit terangkat).
		cls += " hover:bg-base-200"
	}
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
		h.Class("border-t border-base-300 p-3 flex flex-col gap-2"),
		h.Div(
			h.Class("flex items-center justify-between gap-2 min-w-0"),
			h.Div(
				h.Class("flex items-center gap-2 min-w-0"),
				Avatar(d.AvatarURL, "", d.UserEmail, 32),
				h.Span(h.Class("app-navlabel text-sm truncate"), g.Text(d.UserEmail)),
			),
			ThemeToggleUp(),
		),
		h.Button(
			h.Class("btn btn-outline btn-sm w-full"),
			g.Attr("title", "Keluar"),
			ConfirmTrigger("logoutConfirm"), // buka modal, bukan langsung logout
			lucide.LogOut(h.Class("size-4")),
			h.Span(h.Class("app-navlabel"), g.Text("Keluar")),
		),
	)
}
