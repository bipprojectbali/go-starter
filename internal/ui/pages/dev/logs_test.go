package dev

import (
	"encoding/json"
	"strings"
	"testing"

	g "maragu.dev/gomponents"
)

func renderNode(t *testing.T, n g.Node) string {
	t.Helper()
	var sb strings.Builder
	if err := n.Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

func sampleLogs() LogsData {
	return LogsData{
		Range:       "day",
		RangeLabel:  "Hari ini",
		IsDaily:     true,
		ActiveUsers: 5,
		TotalHits:   123,
		PeakHour:    "14:00",
		ChartJSON:   `{"series":[{"type":"bar","data":[1,2,3]}]}`,
		Spans: []SpanRow{
			{Email: "a@x.com", FirstSeen: "02 Jul 09:00", LastSeen: "02 Jul 17:00", Hits: 40},
		},
		AuthEvents: []AuthRow{
			{Action: "auth.login", Method: "google", UserID: 7, When: "02 Jul 09:00"},
			{Action: "auth.logout", Method: "", UserID: 7, When: "02 Jul 17:00"},
		},
	}
}

// TestLogsPage_ChartDataIsCSPSafeJSON: data chart ditanam sebagai
// <script type="application/json"> (bukan inline JS) — TIDAK dieksekusi browser,
// lolos CSP script-src 'self'. Ini inti keputusan arsitektur (bukan go-echarts inline).
func TestLogsPage_ChartDataIsCSPSafeJSON(t *testing.T) {
	out := renderNode(t, LogsPage(sampleLogs()))

	if !strings.Contains(out, `type="application/json"`) {
		t.Errorf("data chart harus di <script type=application/json>:\n%s", out)
	}
	if !strings.Contains(out, `id="chart-activity-data"`) {
		t.Errorf("id data chart (daily) harus ada:\n%s", out)
	}
	// Runtime + init dimuat dari /static (same-origin, CSP-safe).
	if !strings.Contains(out, "/static/echarts.min.js") || !strings.Contains(out, "/static/charts.js") {
		t.Errorf("echarts + charts.js harus dimuat:\n%s", out)
	}
	// JSON option ter-render apa adanya (g.Raw, bukan escaped) → charts.js bisa JSON.parse.
	if !strings.Contains(out, `{"series":[{"type":"bar"`) {
		t.Errorf("option JSON harus ter-render mentah:\n%s", out)
	}
}

func TestLogsPage_KPIandTabs(t *testing.T) {
	out := renderNode(t, LogsPage(sampleLogs()))
	for _, want := range []string{
		"User aktif", "123", "14:00", // KPI values
		`href="/dev/logs?range=day"`, `href="/dev/logs?range=week"`, `href="/dev/logs?range=month"`,
		"tab-active", // tab harian aktif
		"a@x.com",    // span row
		"stat-value", // daisyUI stats dipakai
	} {
		if !strings.Contains(out, want) {
			t.Errorf("LogsPage kurang %q:\n%s", want, out)
		}
	}
}

// TestLogsPage_TrendUsesTrendChartID: rentang non-harian pakai id chart-trend.
func TestLogsPage_TrendUsesTrendChartID(t *testing.T) {
	d := sampleLogs()
	d.IsDaily = false
	d.Range = "week"
	out := renderNode(t, LogsPage(d))
	if !strings.Contains(out, `id="chart-trend-data"`) {
		t.Errorf("rentang mingguan harus pakai chart-trend:\n%s", out)
	}
}

// TestLogsPage_EmptyStates: tabel kosong menampilkan pesan, bukan crash.
func TestLogsPage_EmptyStates(t *testing.T) {
	d := sampleLogs()
	d.Spans = nil
	d.AuthEvents = nil
	out := renderNode(t, LogsPage(d))
	if !strings.Contains(out, "Belum ada aktivitas") {
		t.Errorf("span kosong harus tampilkan pesan:\n%s", out)
	}
	if !strings.Contains(out, "Belum ada event") {
		t.Errorf("auth kosong harus tampilkan pesan:\n%s", out)
	}
}

// TestLogsPage_ChartJSONValid: ChartJSON yang dihasilkan handler valid (guard
// regresi bila builder berubah).
func TestSampleChartJSONParses(t *testing.T) {
	var v any
	if err := json.Unmarshal([]byte(sampleLogs().ChartJSON), &v); err != nil {
		t.Fatalf("ChartJSON contoh harus JSON valid: %v", err)
	}
}
