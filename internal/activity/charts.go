package activity

import "strconv"

// EChartsOption adalah option chart ECharts sebagai struktur generik (map) yang
// di-marshal jadi JSON lalu ditanam di <script type="application/json"> dan
// dibaca charts.js. Kita bangun di Go (bukan JS) agar bisa di-test & data
// menyatu dengan render server. Tipe alias supaya ringkas.
type EChartsOption = map[string]any

// HourlyBarOption membangun option bar "aktivitas per jam" (0..23 lokal).
// series total_hits (batang) — fokus volume aktivitas per jam.
func HourlyBarOption(points []HourPoint) EChartsOption {
	hours := make([]string, len(points))
	hits := make([]int64, len(points))
	users := make([]int64, len(points))
	for i, p := range points {
		hours[i] = pad2(p.Hour) + ":00"
		hits[i] = p.TotalHits
		users[i] = p.ActiveUsers
	}
	return EChartsOption{
		"tooltip": map[string]any{"trigger": "axis"},
		"legend":  map[string]any{"data": []string{"Aktivitas", "User aktif"}},
		"grid":    map[string]any{"left": "3%", "right": "4%", "bottom": "3%", "containLabel": true},
		"xAxis": map[string]any{
			"type": "category", "data": hours,
			"axisLabel": map[string]any{"interval": 1},
		},
		"yAxis": map[string]any{"type": "value", "minInterval": 1},
		"series": []any{
			map[string]any{"name": "Aktivitas", "type": "bar", "data": hits},
			map[string]any{"name": "User aktif", "type": "line", "smooth": true, "data": users},
		},
	}
}

// DailyTrendOption membangun option line "tren harian" untuk rentang minggu/bulan.
func DailyTrendOption(points []DayPoint) EChartsOption {
	days := make([]string, len(points))
	hits := make([]int64, len(points))
	users := make([]int64, len(points))
	for i, p := range points {
		days[i] = p.Day.Format("02 Jan")
		hits[i] = p.TotalHits
		users[i] = p.ActiveUsers
	}
	return EChartsOption{
		"tooltip": map[string]any{"trigger": "axis"},
		"legend":  map[string]any{"data": []string{"Aktivitas", "User aktif"}},
		"grid":    map[string]any{"left": "3%", "right": "4%", "bottom": "3%", "containLabel": true},
		"xAxis":   map[string]any{"type": "category", "data": days},
		"yAxis":   map[string]any{"type": "value", "minInterval": 1},
		"series": []any{
			map[string]any{"name": "Aktivitas", "type": "line", "smooth": true, "areaStyle": map[string]any{}, "data": hits},
			map[string]any{"name": "User aktif", "type": "line", "smooth": true, "data": users},
		},
	}
}

func pad2(n int) string {
	s := strconv.Itoa(n)
	if len(s) < 2 {
		return "0" + s
	}
	return s
}
