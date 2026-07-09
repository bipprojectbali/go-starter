// Package activity menghitung rentang waktu & meng-agregasi data presence jadi
// bentuk siap-render (termasuk option ECharts). Pure & testable — tak menyentuh
// DB/HTTP. Handler memetakan baris sqlc ke tipe di sini lalu memformat output.
package activity

import "time"

// Range = granularitas filter panel (harian/mingguan/bulanan).
type Range string

const (
	RangeDay   Range = "day"
	RangeWeek  Range = "week"
	RangeMonth Range = "month"
)

// ParseRange memetakan string query (?range=) ke Range; default harian.
func ParseRange(s string) Range {
	switch s {
	case "week":
		return RangeWeek
	case "month":
		return RangeMonth
	default:
		return RangeDay
	}
}

// Label mengembalikan label manusiawi (Indonesia) untuk rentang.
func (r Range) Label() string {
	switch r {
	case RangeWeek:
		return "7 hari terakhir"
	case RangeMonth:
		return "30 hari terakhir"
	default:
		return "Hari ini"
	}
}

// Window menghitung batas [from, to) dalam UTC untuk rentang, dihitung dari now
// pada zona lokal loc. day = sejak tengah malam lokal hari ini; week/month =
// N hari ke belakang mulai tengah malam lokal. now di-parameter agar testable.
func (r Range) Window(now time.Time, loc *time.Location) (from, to time.Time) {
	if loc == nil {
		loc = time.UTC
	}
	local := now.In(loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	switch r {
	case RangeWeek:
		from = midnight.AddDate(0, 0, -6) // termasuk hari ini = 7 hari
	case RangeMonth:
		from = midnight.AddDate(0, 0, -29) // 30 hari
	default:
		from = midnight
	}
	to = midnight.AddDate(0, 0, 1) // eksklusif: awal besok lokal
	return from.UTC(), to.UTC()
}

// IsDaily melaporkan apakah rentang memakai grafik per-jam (day) vs per-hari.
func (r Range) IsDaily() bool { return r == RangeDay }

// HourPoint = satu titik distribusi per jam (0..23) lokal.
type HourPoint struct {
	Hour        int
	ActiveUsers int64
	TotalHits   int64
}

// FillHours memastikan 24 titik (0..23) ada; jam tanpa data → 0. present = data
// dari query (hanya jam yang punya baris). ECharts butuh sumbu X lengkap.
func FillHours(present []HourPoint) []HourPoint {
	byHour := make(map[int]HourPoint, len(present))
	for _, p := range present {
		byHour[p.Hour] = p
	}
	out := make([]HourPoint, 24)
	for h := 0; h < 24; h++ {
		if p, ok := byHour[h]; ok {
			out[h] = p
		} else {
			out[h] = HourPoint{Hour: h}
		}
	}
	return out
}

// DayPoint = satu titik tren per hari lokal.
type DayPoint struct {
	Day         time.Time // tengah malam lokal
	ActiveUsers int64
	TotalHits   int64
}

// FillDays memastikan tiap hari dalam [from, to) lokal punya titik (0 bila kosong).
// from/to = batas lokal (tengah malam). present dipetakan by tanggal (YYYY-MM-DD).
func FillDays(present []DayPoint, from, to time.Time) []DayPoint {
	byDay := make(map[string]DayPoint, len(present))
	for _, p := range present {
		byDay[p.Day.Format("2006-01-02")] = p
	}
	var out []DayPoint
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		if p, ok := byDay[key]; ok {
			out = append(out, p)
		} else {
			out = append(out, DayPoint{Day: d})
		}
	}
	return out
}
