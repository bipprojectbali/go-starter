package dev

import (
	"strings"
	"testing"

	"go_stater/internal/health"
)

func sampleResult() health.Result {
	return health.Result{
		Total:     2,
		Unhealthy: 1,
		Reports: []health.Report{
			{Path: "internal/handler/big.go", Kind: "Route/Handler", Lines: 300, Chars: 9000, Limit: 150, Healthy: false, Reasons: []string{"baris melebihi ambang tipe"}},
			{Path: "internal/auth/ok.go", Kind: "Service/Lainnya", Lines: 80, Chars: 2000, Limit: 300, Healthy: true},
		},
	}
}

func TestHealthPage_ToolbarAndControls(t *testing.T) {
	var sb strings.Builder
	if err := HealthPage(sampleResult()).Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := sb.String()

	for _, want := range []string{
		`id="health-panel"`,
		`id="health-q"`,                        // search
		`id="health-status"`,                   // filter status
		`id="health-kind"`,                     // filter tipe
		`id="health-select-all"`,               // pilih semua
		`id="health-copy-selected"`,            // copy terpilih
		`id="health-copy-unhealthy"`,           // copy bermasalah
		`id="health-prev"`, `id="health-next"`, // pagination prev/next
		`id="health-pages"`, // wadah tombol angka halaman (diisi JS)
		`data-health-row`,   // baris terfilter JS
		`data-health-check`, // checkbox per baris
		`/static/health.js`, // logika JS dimuat
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HealthPage kurang %q", want)
		}
	}
}

func TestHealthPage_RowDataAttributes(t *testing.T) {
	var sb strings.Builder
	HealthPage(sampleResult()).Render(&sb)
	out := sb.String()

	// Baris membawa path/status/kind untuk difilter JS.
	if !strings.Contains(out, `data-path="internal/handler/big.go"`) {
		t.Error("baris harus punya data-path")
	}
	if !strings.Contains(out, `data-status="unhealthy"`) || !strings.Contains(out, `data-status="healthy"`) {
		t.Error("baris harus punya data-status healthy & unhealthy")
	}
	// Dropdown tipe berisi kind yang ada.
	if !strings.Contains(out, "Route/Handler") || !strings.Contains(out, "Service/Lainnya") {
		t.Error("dropdown tipe harus berisi kind dari hasil scan")
	}
}

func TestHealthPage_UnhealthySummary(t *testing.T) {
	var sb strings.Builder
	HealthPage(sampleResult()).Render(&sb)
	out := sb.String()
	if !strings.Contains(out, "1 dari 2 file perlu perhatian") {
		t.Errorf("ringkasan tak sehat salah:\n%s", out)
	}
}
