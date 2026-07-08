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
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Database ERD")),
		h.P(h.Class("text-sm text-muted-foreground mb-4"),
			g.Text(strconv.Itoa(tableCount)+" tabel — relasi & kolom dari katalog live Postgres. Hanya dev.")),
		h.Div(
			h.Class("card"),
			h.Section(
				h.Class("overflow-auto"),
				// Sumber diagram: <pre class="mermaid"> berisi teks erDiagram.
				// Mermaid mengganti isinya dengan SVG saat init. g.Text meng-escape
				// aman; teks Mermaid tak mengandung HTML.
				h.Pre(h.Class("mermaid"), h.ID("erd-diagram"), g.Text(mermaidSrc)),
			),
		),
		// Runtime Mermaid (vendored, ~3MB) + init (keduanya same-origin, patuh CSP).
		h.Script(h.Src("/static/mermaid.min.js")),
		h.Script(h.Src("/static/erd.js"), h.Defer()),
	)
}
