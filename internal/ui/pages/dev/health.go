package dev

import (
	"strconv"
	"strings"

	"go_stater/internal/health"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// HealthPage merender ringkasan + tabel kesehatan file. File tak sehat di atas
// (sudah terurut oleh health.Scan).
func HealthPage(res health.Result) g.Node {
	summary := "Semua " + strconv.Itoa(res.Total) + " file sehat ✓"
	summaryVariant := "" // hijau (default)
	if res.Unhealthy > 0 {
		summary = strconv.Itoa(res.Unhealthy) + " dari " + strconv.Itoa(res.Total) + " file perlu perhatian"
		summaryVariant = "destructive"
	}

	return h.Div(
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("File Health")),
		h.P(h.Class("text-sm text-muted-foreground mb-4"),
			g.Text("Ambang: per-tipe (handler 150, service 300, dst) + hard limit 500 baris / 20.000 karakter. Hanya dev.")),
		healthSummary(summary, summaryVariant),
		h.Div(
			h.Class("card mt-4"),
			h.Section(
				h.Class("overflow-x-auto"),
				h.Table(
					h.Class("w-full text-sm"),
					h.THead(
						h.Tr(
							h.Class("border-b text-left text-muted-foreground"),
							th("File"), th("Tipe"), th("Baris"), th("Karakter"), th("Status"),
						),
					),
					h.TBody(g.Map(res.Reports, healthRow)),
				),
			),
		),
	)
}

func healthSummary(msg, variant string) g.Node {
	attrs := []g.Node{h.Class("alert"), h.Role("status")}
	if variant != "" {
		attrs = append(attrs, g.Attr("data-variant", variant))
	}
	return h.Div(append(attrs, g.Text(msg))...)
}

func healthRow(r health.Report) g.Node {
	attrs := []g.Node{h.Class("border-b")}
	if !r.Healthy {
		// Aksen kiri merah (inline style — token destructive tak ada di app.css theme).
		attrs = append(attrs, g.Attr("style", "box-shadow: inset 3px 0 0 var(--color-destructive, #ef4444)"))
	}
	// Baris "N / limit".
	lineCell := strconv.Itoa(r.Lines) + " / " + strconv.Itoa(r.Limit)
	cells := []g.Node{
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
	// Alasan digabung sbg tooltip.
	return h.Span(
		h.Class("badge"),
		g.Attr("data-variant", "destructive"),
		h.Title(strings.Join(r.Reasons, "; ")),
		g.Text("perlu perhatian"),
	)
}
