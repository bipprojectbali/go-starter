package ui

import (
	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// ThemeOption = satu tema daisyUI yang bisa dipilih user. Label = teks tampil,
// Value = nama tema (harus terdaftar di @plugin daisyui.js di input.css).
type ThemeOption struct {
	Label string
	Value string
	Dark  bool // untuk ikon indikator; tak memengaruhi CSS
}

// themeList = daftar tema kurasi (SATU sumber kebenaran). Harus SELARAS dengan
// blok `themes:` di static/input.css — tema yang tak terdaftar di sana tak
// ter-generate CSS-nya. Campuran terang & gelap agar user punya pilihan nyata.
var themeList = []ThemeOption{
	{Label: "Terang", Value: "light"},
	{Label: "Gelap", Value: "dark", Dark: true},
	{Label: "Cupcake", Value: "cupcake"},
	{Label: "Nord", Value: "nord"},
	{Label: "Dim", Value: "dim", Dark: true},
	{Label: "Sunset", Value: "sunset", Dark: true},
}

// ThemeToggle merender pemilih tema yang buka ke BAWAH (tombol ikon di nav atas
// Layout / halaman login). Varian sidebar bergaya baris menu = ThemeMenu.
func ThemeToggle() g.Node { return themeDropdown() }

// themeBlock = blok pemilih tema di sidebar, tepat di bawah pintasan panel dan di
// atas footer identitas. Wrapper selaras quickLinks/notifBlock (border-t + padding)
// agar seragam sebagai satu baris menu.
func themeBlock() g.Node {
	return h.Div(
		h.Class("border-t border-base-300 px-3 py-2 flex flex-col gap-1"),
		ThemeMenu(),
	)
}

// ThemeMenu merender pemilih tema sebagai BARIS menu sidebar (bukan tombol ikon):
// selaras navLink (ikon + label + indikator), full-width, membuka ke ATAS. Dipakai
// di blok tema sidebar — SENGAJA terpisah dari footer identitas user agar grouping
// tak bercampur (Tema = preferensi TAMPILAN, bukan aksi akun). Menutupi keluhan
// email panjang menumpuk dengan ikon tema saat keduanya sebaris.
func ThemeMenu() g.Node {
	items := make([]g.Node, 0, len(themeList))
	for _, t := range themeList {
		items = append(items, themeRadioItem(t))
	}
	return h.Details(
		h.Class("dropdown dropdown-top w-full"),
		g.Attr("data-theme-dropdown", "true"), // hook theme.js: tutup setelah pilih
		h.Summary(
			// Gaya = navLink non-aktif (lihat shellnav.go) agar seragam dgn menu lain.
			// display:flex sekaligus menyingkirkan segitiga <summary> bawaan.
			h.Class("app-navlink flex flex-nowrap items-center gap-3 rounded-md border-l-2 border-transparent px-3 py-2 text-sm hover:bg-base-200 w-full cursor-pointer list-none"),
			g.Attr("aria-label", "Ganti tema"),
			h.Title("Tema"),
			lucide.Palette(h.Class("size-4 shrink-0")),
			h.Span(h.Class("app-navlabel truncate flex-1"), g.Text("Tema")),
			lucide.ChevronUp(h.Class("app-navlabel size-4 shrink-0 opacity-60")),
		),
		h.Ul(
			h.Class("dropdown-content menu bg-base-100 border border-base-300 rounded-box z-50 mb-2 w-40 p-2 shadow-lg"),
			g.Group(items),
		),
	)
}

// themeDropdown = pemilih tema (dropdown CSS-only via <details>, CSP-safe —
// tanpa inline JS) sebagai tombol ikon yang buka ke BAWAH. Tiap opsi = radio
// ber-class `theme-controller`; daisyUI menerapkan tema dari radio yang tercentang
// lewat CSS murni (`:has()`). Persistensi & no-FOUC ditangani theme.js (set
// data-theme + centang radio yang cocok saat load). name grup unik agar radio
// saling eksklusif.
func themeDropdown() g.Node {
	items := make([]g.Node, 0, len(themeList))
	for _, t := range themeList {
		items = append(items, themeRadioItem(t))
	}
	// Menu pakai base-100 + border → kontras jelas di atas latar base-200
	// maupun sidebar base-100 (shadow + border memberi batas), di semua tema.
	return h.Details(
		h.Class("dropdown dropdown-end"),
		g.Attr("data-theme-dropdown", "true"), // hook theme.js: tutup setelah pilih
		h.Summary(
			h.Class("btn btn-ghost btn-sm gap-1"),
			g.Attr("aria-label", "Ganti tema"),
			g.Attr("title", "Ganti tema"),
			lucide.Palette(h.Class("size-4")),
			h.Span(h.Class("app-navlabel"), g.Text("Tema")),
		),
		h.Ul(
			h.Class("dropdown-content menu bg-base-100 border border-base-300 rounded-box z-50 mt-2 w-40 p-2 shadow-lg"),
			g.Group(items),
		),
	)
}

// themeRadioItem = satu baris tema dalam dropdown. Radio theme-controller +
// ikon indikator terang/gelap. Value dikirim ke daisyUI (CSS) & disimpan
// theme.js (via listener change → localStorage).
func themeRadioItem(t ThemeOption) g.Node {
	icon := lucide.Sun(h.Class("size-4 opacity-70"))
	if t.Dark {
		icon = lucide.Moon(h.Class("size-4 opacity-70"))
	}
	return h.Li(
		h.Label(
			h.Class("flex items-center gap-2 cursor-pointer"),
			h.Input(
				h.Type("radio"),
				h.Name("theme-choice"),
				h.Class("radio radio-sm theme-controller"),
				h.Value(t.Value),
				g.Attr("aria-label", t.Label),
			),
			icon,
			h.Span(g.Text(t.Label)),
		),
	)
}
