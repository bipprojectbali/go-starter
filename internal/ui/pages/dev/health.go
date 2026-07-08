package dev

import (
	"sort"
	"strconv"
	"strings"

	"go_stater/internal/health"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// HealthPage merender ringkasan + toolbar filter + tabel + pagination. Filter,
// pagination, dan copy dikerjakan client-side oleh /static/health.js (dataset
// kecil, semua baris dikirim sekali). Baris membawa atribut data-* yang dibaca JS.
func HealthPage(res health.Result) g.Node {
	summary := "Semua " + strconv.Itoa(res.Total) + " file sehat ✓"
	summaryVariant := ""
	if res.Unhealthy > 0 {
		summary = strconv.Itoa(res.Unhealthy) + " dari " + strconv.Itoa(res.Total) + " file perlu perhatian"
		summaryVariant = "destructive"
	}

	return h.Div(
		h.ID("health-panel"),
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("File Health")),
		h.P(h.Class("text-sm text-muted-foreground mb-4"),
			g.Text("Ambang: per-tipe (handler 150, service 300, dst) + hard limit 500 baris / 20.000 karakter. Hanya dev.")),
		healthSummary(summary, summaryVariant),
		healthToolbar(res),
		h.Div(
			h.Class("card mt-4"),
			h.Section(
				h.Class("overflow-x-auto"),
				h.Table(
					h.Class("w-full text-sm"),
					h.THead(
						h.Tr(
							h.Class("border-b text-left text-muted-foreground"),
							h.Th(h.Class("py-2 pr-4 w-8"),
								h.Input(h.Type("checkbox"), h.ID("health-select-all"), h.Title("Pilih halaman ini")),
							),
							th("File"), th("Tipe"), th("Baris"), th("Karakter"), th("Status"),
						),
					),
					h.TBody(g.Map(res.Reports, healthRow)),
				),
			),
		),
		healthPagination(),
		// Toast copy (client-side, fade via inline style transition).
		h.Div(
			h.ID("health-toast"),
			h.Class("fixed bottom-4 right-4 z-50 alert shadow-lg"),
			g.Attr("style", "opacity:0; transition:opacity .2s"),
			h.Role("status"),
		),
		// Logika filter/pagination/copy (same-origin, patuh CSP script-src 'self').
		h.Script(h.Src("/static/health.js"), h.Defer()),
	)
}

func healthSummary(msg, variant string) g.Node {
	attrs := []g.Node{h.Class("alert"), h.Role("status")}
	if variant != "" {
		attrs = append(attrs, g.Attr("data-variant", variant))
	}
	return h.Div(append(attrs, g.Text(msg))...)
}

// healthToolbar = kontrol filter (search + status + tipe) & tombol copy.
func healthToolbar(res health.Result) g.Node {
	return h.Div(
		h.Class("mt-4 flex flex-wrap items-center gap-2"),
		h.Input(
			h.Type("search"),
			h.ID("health-q"),
			h.Class("input max-w-xs"),
			h.Placeholder("Cari nama/path file…"),
		),
		h.Select(
			h.ID("health-status"),
			h.Class("input w-auto"),
			h.Option(h.Value(""), g.Text("Semua status")),
			h.Option(h.Value("unhealthy"), g.Text("Perlu perhatian")),
			h.Option(h.Value("healthy"), g.Text("Sehat")),
		),
		kindSelect(res),
		h.Div(h.Class("flex-1")), // spacer
		h.Button(
			h.Type("button"), h.ID("health-copy-selected"),
			h.Class("btn"), g.Attr("data-variant", "outline"), g.Attr("data-size", "sm"),
			g.Text("Copy terpilih"),
		),
		h.Button(
			h.Type("button"), h.ID("health-copy-unhealthy"),
			h.Class("btn"), g.Attr("data-variant", "outline"), g.Attr("data-size", "sm"),
			g.Text("Copy bermasalah"),
		),
	)
}

// kindSelect membangun dropdown tipe dari kind unik yang ada di hasil scan.
func kindSelect(res health.Result) g.Node {
	seen := map[string]bool{}
	var kinds []string
	for _, r := range res.Reports {
		if !seen[r.Kind] {
			seen[r.Kind] = true
			kinds = append(kinds, r.Kind)
		}
	}
	sort.Strings(kinds)
	opts := []g.Node{h.Option(h.Value(""), g.Text("Semua tipe"))}
	for _, k := range kinds {
		opts = append(opts, h.Option(h.Value(k), g.Text(k)))
	}
	return h.Select(append([]g.Node{h.ID("health-kind"), h.Class("input w-auto")}, opts...)...)
}

func healthPagination() g.Node {
	return h.Div(
		h.Class("mt-3 flex items-center justify-between text-sm"),
		h.Span(h.ID("health-page-info"), h.Class("text-muted-foreground")),
		h.Div(
			h.Class("flex gap-2"),
			h.Button(h.Type("button"), h.ID("health-prev"),
				h.Class("btn"), g.Attr("data-variant", "outline"), g.Attr("data-size", "sm"),
				g.Text("Sebelumnya")),
			h.Button(h.Type("button"), h.ID("health-next"),
				h.Class("btn"), g.Attr("data-variant", "outline"), g.Attr("data-size", "sm"),
				g.Text("Berikutnya")),
		),
	)
}

func healthRow(r health.Report) g.Node {
	status := "healthy"
	if !r.Healthy {
		status = "unhealthy"
	}
	attrs := []g.Node{
		h.Class("border-b"),
		g.Attr("data-health-row", "true"),
		g.Attr("data-path", r.Path),
		g.Attr("data-status", status),
		g.Attr("data-kind", r.Kind),
	}
	if !r.Healthy {
		attrs = append(attrs, g.Attr("style", "box-shadow: inset 3px 0 0 var(--color-destructive, #ef4444)"))
	}
	lineCell := strconv.Itoa(r.Lines) + " / " + strconv.Itoa(r.Limit)
	cells := []g.Node{
		h.Td(h.Class("py-2 pr-4"),
			h.Input(h.Type("checkbox"), g.Attr("data-health-check", "true"), h.Title("Pilih file ini")),
		),
		h.Td(h.Class("py-2 pr-4 font-mono text-xs"), g.Text(r.Path)),
		h.Td(h.Class("py-2 pr-4 text-muted-foreground"), g.Text(r.Kind)),
		h.Td(h.Class("py-2 pr-4"), g.Text(lineCell)),
		h.Td(h.Class("py-2 pr-4"), g.Text(strconv.Itoa(r.Chars))),
		h.Td(h.Class("py-2"), healthStatus(r)),
	}
	return h.Tr(append(attrs, cells...)...)
}

func healthStatus(r health.Report) g.Node {
	if r.Healthy {
		return badge("sehat", "")
	}
	return h.Span(
		h.Class("badge"),
		g.Attr("data-variant", "destructive"),
		h.Title(strings.Join(r.Reasons, "; ")),
		g.Text("perlu perhatian"),
	)
}
