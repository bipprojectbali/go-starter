package ui

// shellbrand.go — blok brand di header sidebar: nama workspace, penanda panel
// (chip + aksen tepi), dan dropdown pindah workspace.
//
// Dipisah dari appshell.go karena ini concern IDENTITAS (workspace mana + panel
// mana), sementara appshell.go adalah kerangka layout. Gaya panel sendiri ada di
// panelkind.go; di sini hanya perenderannya.

import (
	"strconv"

	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// shellBrand merender blok brand di header sidebar: nama workspace sebagai baris
// utama + konteks panel (BrandLabel, mis. "go_starter /admin") sebagai sub-label
// kecil. Bila WorkspaceName kosong (platform tanpa konteks tenant), tampilkan
// hanya BrandLabel (fallback perilaku lama). Kelas app-navlabel disembunyikan saat
// rail collapsed (konsisten label lain).
func shellBrand(d ShellData) g.Node {
	if d.WorkspaceName == "" {
		return h.Div(
			h.Class("app-brand flex-1 min-w-0 flex flex-col justify-center leading-tight"),
			h.Span(h.Class("font-semibold truncate"), g.Text(d.BrandLabel)),
			panelChip(d.Panel),
		)
	}
	label := h.Div(
		h.Class("min-w-0 flex flex-col justify-center leading-tight text-left"),
		h.Span(h.Class("font-semibold truncate"), g.Text(d.WorkspaceName)),
		// Baris kedua = identitas PANEL. Chip berwarna bila panel dikenal; kalau
		// tidak, jatuh ke sub-label lama (halaman di luar ketiga panel).
		panelSubLabel(d),
	)
	// Satu workspace & tak bisa buat baru → teks biasa (tak perlu dropdown).
	if len(d.Workspaces) <= 1 && !d.CanCreateWorkspace {
		return h.Div(h.Class("app-brand flex-1 min-w-0"), label)
	}
	return h.Div(h.Class("app-brand flex-1 min-w-0"), workspaceSwitcher(d, label))
}

// panelSubLabel = baris kedua brand: chip panel bila dikenal, selainnya
// sub-label teks lama (mis. halaman di luar ketiga panel).
func panelSubLabel(d ShellData) g.Node {
	if d.Panel.style().Label != "" {
		return panelChip(d.Panel)
	}
	return h.Span(h.Class("app-navlabel text-xs text-base-content/80 truncate"), g.Text(d.BrandLabel))
}

// panelChip = penanda TEKS panel yang sedang dibuka. Teks + warna sekaligus:
// yang tak menangkap warna tetap membaca "PLATFORM". Saat sidebar collapse jadi
// rail 4rem, chip ikut tersembunyi lewat induknya `.app-brand` (input.css) —
// itulah kenapa aksen tepi ada: penanda tetap hidup saat rail (terverifikasi).
func panelChip(p Panel) g.Node {
	st := p.style()
	if st.Label == "" {
		return g.Text("")
	}
	return h.Span(
		h.Class("app-navlabel badge badge-xs "+st.Chip+" mt-0.5 w-fit font-medium tracking-wide"),
		g.Text(st.Label),
	)
}

// panelEdge = garis aksen tipis di tepi ATAS sidebar. Penanda yang tertangkap
// tanpa membaca, dan tetap terlihat saat sidebar jadi rail ikon (saat itu chip
// tersembunyi) — jadi keduanya saling menutupi, bukan mengulang.
func panelEdge(p Panel) g.Node {
	st := p.style()
	if st.Edge == "" {
		return g.Text("")
	}
	return h.Div(h.Class("h-1 shrink-0 " + st.Edge))
}

// workspaceSwitcher = dropdown pindah workspace (model membership: satu user bisa
// anggota banyak workspace). Pola <details> CSS-only sama seperti themeDropdown —
// CSP-safe, tanpa inline JS. Pindah = native form POST → 303 (gotcha #16: redirect
// via SSE menyuntik <script> yang diblokir CSP).
func workspaceSwitcher(d ShellData, label g.Node) g.Node {
	items := make([]g.Node, 0, len(d.Workspaces)+1)
	for _, ws := range d.Workspaces {
		items = append(items, workspaceSwitchItem(ws, ws.TenantID == d.ActiveTenantID))
	}
	if d.CanCreateWorkspace {
		items = append(items,
			h.Li(h.Class("border-t border-base-300 mt-1 pt-1"),
				h.A(h.Href("/workspace/new"), h.Class("gap-2"),
					lucide.Plus(h.Class("size-4")),
					g.Text("Buat workspace baru"),
				),
			),
		)
	}
	return h.Details(
		h.Class("dropdown w-full"),
		h.Summary(
			h.Class("btn btn-ghost btn-sm w-full justify-between gap-1 px-1 h-auto py-1"),
			g.Attr("aria-label", "Ganti workspace"),
			label,
			lucide.ChevronsUpDown(h.Class("size-4 shrink-0 opacity-60")),
		),
		h.Ul(
			h.Class("dropdown-content menu bg-base-100 border border-base-300 rounded-box z-50 mt-2 w-56 p-2 shadow-lg"),
			g.Group(items),
		),
	)
}

// workspaceSwitchItem = satu workspace di dropdown. Aktif → ditandai + tak bisa
// diklik (sudah di sana). Lainnya = tombol submit form POST /workspace/switch.
func workspaceSwitchItem(ws WorkspaceOption, active bool) g.Node {
	name := h.Div(
		h.Class("flex flex-col items-start leading-tight min-w-0"),
		h.Span(h.Class("truncate"), g.Text(ws.Name)),
		h.Span(h.Class("text-xs text-base-content/80"), g.Text(ws.Role)),
	)
	if active {
		return h.Li(h.Div(
			h.Class("flex items-center justify-between gap-2 bg-primary/10 rounded-md"),
			name,
			lucide.Check(h.Class("size-4 shrink-0 text-primary")),
		))
	}
	return h.Li(h.FormEl(
		h.Method("post"), h.Action("/workspace/switch"),
		h.Input(h.Type("hidden"), h.Name("tenant"), h.Value(strconv.FormatInt(ws.TenantID, 10))),
		h.Button(h.Type("submit"), h.Class("btn btn-ghost btn-sm w-full justify-start font-normal"), name),
	))
}
