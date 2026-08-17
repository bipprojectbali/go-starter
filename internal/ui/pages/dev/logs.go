package dev

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

// LogsData = view-model panel aktivitas (siap-render; handler yang menghitung).
type LogsData struct {
	Range       string    // "day" | "week" | "month" (untuk active tab)
	RangeLabel  string    // teks manusiawi (mis. "Hari ini")
	IsDaily     bool      // true → chart per-jam; false → tren harian
	ActiveUsers int64     // KPI: user unik aktif di rentang
	TotalHits   int64     // KPI: total aktivitas
	PeakHour    string    // KPI: jam tersibuk (mis. "14:00"); "" bila tak ada
	ChartJSON   string    // option ECharts (bar/line) sudah di-marshal
	Spans       []SpanRow // tabel "aktif jam berapa s/d jam berapa"
	// Trail = jejak SEMUA aksi (audit_logs), menggantikan tabel login/logout.
	// Presence & audit sengaja tetap dua tabel terpisah di halaman yang sama:
	// yang satu menjawab "kapan orang ada" (agregat), yang lain "siapa melakukan
	// apa" (bukti per-peristiwa). Digabung, keduanya jadi tak berguna.
	Trail TrailView
}

// SpanRow = satu baris rentang aktivitas user (first/last seen sudah diformat lokal).
type SpanRow struct {
	Email     string
	FirstSeen string
	LastSeen  string
	Hits      int64
}

// LogsPage merender dashboard aktivitas: KPI + tab rentang + chart + tabel.
func LogsPage(d LogsData) g.Node {
	chartID := "chart-trend"
	if d.IsDaily {
		chartID = "chart-activity"
	}
	return h.Div(
		h.H1(h.Class("text-xl font-semibold mb-1"), g.Text("User Activity")),
		h.P(h.Class("text-sm text-base-content/80 mb-4"),
			g.Text("Kehadiran user (presence) & jejak siapa melakukan apa. "+
				"Waktu ditampilkan zona lokal aplikasi.")),

		rangeTabs(d.Range),
		kpiCards(d),
		activityChart(chartID, d.ChartJSON),
		spansCard(d.Spans),
		TrailCard(d.Trail),

		// Runtime ECharts (vendored) + init (keduanya same-origin → CSP-safe).
		h.Script(h.Src("/static/echarts.min.js")),
		h.Script(h.Src("/static/charts.js"), h.Defer()),
	)
}

// rangeTabs = filter harian/mingguan/bulanan sebagai link (full-page reload).
func rangeTabs(current string) g.Node {
	tab := func(val, label string) g.Node {
		cls := "tab"
		if val == current {
			cls = "tab tab-active"
		}
		return h.A(h.Href("/dev/logs?range="+val), h.Role("tab"), h.Class(cls), g.Text(label))
	}
	return h.Div(
		h.Role("tablist"), h.Class("tabs tabs-box mb-4 w-fit"),
		tab("day", "Harian"),
		tab("week", "Mingguan"),
		tab("month", "Bulanan"),
	)
}

// kpiCards = ringkasan angka (daisyUI stats).
func kpiCards(d LogsData) g.Node {
	peak := d.PeakHour
	if peak == "" {
		peak = "—"
	}
	return h.Div(
		h.Class("stats stats-vertical sm:stats-horizontal bg-base-100 border border-base-300 mb-4 w-full"),
		stat("User aktif", itoa64(d.ActiveUsers), d.RangeLabel),
		stat("Total aktivitas", itoa64(d.TotalHits), d.RangeLabel),
		stat("Jam tersibuk", peak, d.RangeLabel),
	)
}

func stat(title, value, desc string) g.Node {
	return h.Div(
		h.Class("stat"),
		h.Div(h.Class("stat-title"), g.Text(title)),
		h.Div(h.Class("stat-value text-primary"), g.Text(value)),
		h.Div(h.Class("stat-desc"), g.Text(desc)),
	)
}

// activityChart = kontainer chart + data JSON (ECharts option) untuk charts.js.
// JSON ditaruh di <script type="application/json"> — TIDAK dieksekusi browser
// (CSP-safe). g.Text meng-escape aman.
func activityChart(chartID, chartJSON string) g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 mb-4"),
		h.Div(
			h.Class("card-body"),
			h.Div(h.ID(chartID), h.Style("height:320px")),
			h.Script(
				h.Type("application/json"),
				h.ID(chartID+"-data"),
				g.Raw(chartJSON), // JSON valid dari json.Marshal (bukan input user) — aman
			),
		),
	)
}

// spansCard = tabel "user aktif jam berapa s/d jam berapa".
func spansCard(rows []SpanRow) g.Node {
	body := []g.Node{}
	for _, s := range rows {
		body = append(body, h.Tr(
			h.Class("border-b border-base-300"),
			h.Td(h.Class("py-2 pr-4"), g.Text(s.Email)),
			h.Td(h.Class("py-2 pr-4"), g.Text(s.FirstSeen)),
			h.Td(h.Class("py-2 pr-4"), g.Text(s.LastSeen)),
			h.Td(h.Class("py-2"), g.Text(itoa64(s.Hits))),
		))
	}
	return tableCard("Rentang aktivitas user", []string{"User", "Pertama", "Terakhir", "Aktivitas"}, body,
		"Belum ada aktivitas pada rentang ini.")
}

// tableCard membungkus tabel dalam card daisyUI. empty = pesan bila tak ada baris.
func tableCard(title string, headers []string, body []g.Node, empty string) g.Node {
	var content g.Node
	if len(body) == 0 {
		content = h.P(h.Class("text-sm text-base-content/80 py-2"), g.Text(empty))
	} else {
		ths := []g.Node{h.Class("border-b border-base-300 text-left text-base-content/80")}
		for _, hd := range headers {
			ths = append(ths, h.Th(h.Class("py-2 pr-4 font-medium"), g.Text(hd)))
		}
		content = ui.TableScroll(h.Table(
			h.Class("w-full text-sm"),
			h.THead(h.Tr(ths...)),
			h.TBody(g.Group(body)),
		))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 mb-4 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-2"), g.Text(title)),
			content,
		),
	)
}
