package dev

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// ERDPage merender diagram ERD via Mermaid. mermaidSrc = teks erDiagram yang
// digenerate server dari katalog DB; Mermaid (vendored) meng-render-nya jadi
// SVG di klien. mermaid.min.js + erd.js (init) dimuat di akhir.
func ERDPage(mermaidSrc string, tableCount int) g.Node {
	return h.Div(
		h.ID("erd-panel"),
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Database ERD")),
				h.P(h.Class("text-sm text-base-content/80"),
					g.Text(strconv.Itoa(tableCount)+" tabel — relasi & kolom dari katalog live Postgres. Hanya dev.")),
			),
			erdZoomControls(),
		),
		h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			// overflow-auto: saat zoom-in melebihi kontainer, bisa di-scroll.
			h.Div(
				h.Class("card-body overflow-auto"),
				h.ID("erd-viewport"),
				// Sumber diagram: <pre class="mermaid"> berisi teks erDiagram.
				// Mermaid mengganti isinya dengan SVG saat init. g.Text meng-escape
				// aman; teks Mermaid tak mengandung HTML.
				h.Pre(h.Class("mermaid"), h.ID("erd-diagram"),
					g.Attr("style", "transform-origin: top left"),
					g.Text(mermaidSrc)),
			),
		),
		// Runtime Mermaid (vendored, ~3MB) + init/zoom (keduanya same-origin, CSP).
		h.Script(h.Src("/static/mermaid.min.js")),
		h.Script(h.Src("/static/erd.js"), h.Defer()),
	)
}

// erdZoomControls = tombol zoom out / reset / in untuk diagram.
func erdZoomControls() g.Node {
	btn := func(id, label, title string) g.Node {
		return h.Button(
			h.Type("button"), h.ID(id),
			h.Class("btn btn-outline btn-sm"),
			g.Attr("title", title),
			g.Text(label),
		)
	}
	return h.Div(
		h.Class("flex items-center gap-1"),
		btn("erd-zoom-out", "−", "Perkecil"),
		h.Span(h.ID("erd-zoom-level"), h.Class("text-sm text-base-content/80 w-12 text-center"), g.Text("100%")),
		btn("erd-zoom-in", "+", "Perbesar"),
		btn("erd-zoom-reset", "Reset", "Kembalikan 100%"),
	)
}
