package activity

import (
	"testing"
	"time"
)

func TestParseRange(t *testing.T) {
	cases := map[string]Range{"day": RangeDay, "week": RangeWeek, "month": RangeMonth, "": RangeDay, "bogus": RangeDay}
	for in, want := range cases {
		if got := ParseRange(in); got != want {
			t.Errorf("ParseRange(%q) = %v, mau %v", in, got, want)
		}
	}
}

// TestWindow_DayLocalMidnight: rentang harian = [00:00 lokal hari ini, 00:00 besok),
// dikonversi ke UTC. Verifikasi boundary timezone (WIB = UTC+7).
func TestWindow_DayLocalMidnight(t *testing.T) {
	wib := time.FixedZone("WIB", 7*3600)
	// now = 2 Jul 2026 20:30 WIB → hari lokal = 2 Jul.
	now := time.Date(2026, 7, 2, 20, 30, 0, 0, wib)
	from, to := RangeDay.Window(now, wib)

	// from = 2 Jul 00:00 WIB = 1 Jul 17:00 UTC.
	wantFrom := time.Date(2026, 7, 1, 17, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 7, 2, 17, 0, 0, 0, time.UTC)
	if !from.Equal(wantFrom) {
		t.Errorf("from = %v, mau %v", from, wantFrom)
	}
	if !to.Equal(wantTo) {
		t.Errorf("to = %v, mau %v", to, wantTo)
	}
	if from.Location() != time.UTC {
		t.Errorf("from harus UTC, dapat %v", from.Location())
	}
}

// TestWindow_WeekMonthSpan: minggu = 7 hari, bulan = 30 hari (termasuk hari ini).
func TestWindow_WeekMonthSpan(t *testing.T) {
	wib := time.FixedZone("WIB", 7*3600)
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, wib)

	fw, tw := RangeWeek.Window(now, wib)
	if d := tw.Sub(fw); d != 7*24*time.Hour {
		t.Errorf("week span = %v, mau 168h", d)
	}
	fm, tm := RangeMonth.Window(now, wib)
	if d := tm.Sub(fm); d != 30*24*time.Hour {
		t.Errorf("month span = %v, mau 720h", d)
	}
}

// TestFillHours: 24 titik, jam tanpa data → 0, jam berdata dipertahankan.
func TestFillHours(t *testing.T) {
	got := FillHours([]HourPoint{
		{Hour: 9, ActiveUsers: 3, TotalHits: 42},
		{Hour: 14, ActiveUsers: 1, TotalHits: 7},
	})
	if len(got) != 24 {
		t.Fatalf("harus 24 titik, dapat %d", len(got))
	}
	if got[9].TotalHits != 42 || got[9].ActiveUsers != 3 {
		t.Errorf("jam 9 harus dipertahankan: %+v", got[9])
	}
	if got[0].TotalHits != 0 {
		t.Errorf("jam 0 tanpa data harus 0: %+v", got[0])
	}
	for h := 0; h < 24; h++ {
		if got[h].Hour != h {
			t.Errorf("indeks %d harus Hour=%d, dapat %d", h, h, got[h].Hour)
		}
	}
}

// TestFillDays: tiap hari dalam rentang punya titik; kosong → 0.
func TestFillDays(t *testing.T) {
	loc := time.UTC
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, loc)
	to := time.Date(2026, 7, 4, 0, 0, 0, 0, loc) // 3 hari: 1,2,3
	got := FillDays([]DayPoint{
		{Day: time.Date(2026, 7, 2, 0, 0, 0, 0, loc), TotalHits: 10},
	}, from, to)
	if len(got) != 3 {
		t.Fatalf("harus 3 hari, dapat %d", len(got))
	}
	if got[0].TotalHits != 0 || got[2].TotalHits != 0 {
		t.Errorf("hari kosong harus 0")
	}
	if got[1].TotalHits != 10 {
		t.Errorf("2 Jul harus 10, dapat %d", got[1].TotalHits)
	}
}

// TestHourlyBarOption: option punya struktur ECharts yang benar.
func TestHourlyBarOption(t *testing.T) {
	opt := HourlyBarOption(FillHours([]HourPoint{{Hour: 9, TotalHits: 5, ActiveUsers: 2}}))
	if opt["xAxis"] == nil || opt["series"] == nil {
		t.Errorf("option kurang xAxis/series: %+v", opt)
	}
	series, ok := opt["series"].([]any)
	if !ok || len(series) != 2 {
		t.Errorf("harus 2 series (bar+line), dapat %v", opt["series"])
	}
}
