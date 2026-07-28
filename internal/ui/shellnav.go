package ui

// shellnav.go — rendering menu sidebar: daftar item, active-state, badge.
// Dipisah dari appshell.go (kerangka layout) karena aturan "item mana yang
// menyala" adalah logika tersendiri yang berubah karena alasan berbeda dari
// struktur shell-nya.

import (
	"strconv"
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

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

// notifBlock merender entri Notifikasi + badge jumlah. nil → tak render apa pun.
// Memakai navLink yang sama dengan menu lain agar active-state & hover identik;
// badge ditempel sebagai saudara, bukan cabang render terpisah.
func notifBlock(d ShellData) g.Node {
	if d.Notifications == nil {
		return g.Text("")
	}
	it := d.Notifications.Item
	active := ""
	if it.Href == d.CurrentPath || strings.HasPrefix(d.CurrentPath, it.Href+"/") {
		active = it.Href
	}
	badge := g.Node(g.Text(""))
	if d.Notifications.Count > 0 {
		// app-navlabel: ikut disembunyikan saat sidebar collapse jadi rail ikon
		// (CSS yang sama dengan label menu) — badge tak menggantung sendirian.
		badge = h.Span(
			h.Class("app-navlabel badge badge-primary badge-sm ml-auto"),
			g.Text(strconv.FormatInt(d.Notifications.Count, 10)),
		)
	}
	return h.Div(
		h.Class("border-t border-base-300 px-3 py-2 flex flex-col gap-1"),
		h.Div(h.Class("flex items-center"), navLinkWith(it, active, badge)),
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
	return navLinkWith(it, activeHref, nil)
}

// navLinkWith = navLink + node tambahan di ujung kanan (mis. badge jumlah).
// Satu implementasi styling untuk semua item menu: varian ber-badge tak bisa
// menyimpang dari yang polos.
func navLinkWith(it NavItem, activeHref string, trailing g.Node) g.Node {
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
	if trailing != nil {
		children = append(children, trailing)
	}
	return h.A(append(attrs, children...)...)
}
