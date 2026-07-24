package handler

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"go_stater/internal/activity"
	"go_stater/internal/db"
	"go_stater/internal/ui/pages/dev"

	"github.com/jackc/pgx/v5/pgtype"
)

// recentAuthLimit = jumlah event auth terbaru yang ditampilkan di tabel panel.
const recentAuthLimit = 20

// Builder view-model panel /dev/logs: memetakan baris sqlc → tipe siap-render.
// Dipisah dari dev_logs.go (handler) agar handler tetap ramping (batas 150 baris).

// buildChart merakit JSON option ECharts + peak hour (day). Error di-log; JSON
// minimal "{}" agar charts.js tak crash.
func (h *Handler) buildChart(ctx context.Context, rng activity.Range, fromTS, toTS pgtype.Timestamptz, from, to time.Time) (string, string) {
	tz := appTZ.String()
	var opt activity.EChartsOption
	peak := ""
	if rng.IsDaily() {
		rows, err := h.q(ctx).PresenceByHour(ctx, db.PresenceByHourParams{Tz: tz, FromAt: fromTS, ToAt: toTS})
		if err != nil {
			h.Log.Error("dev logs: by hour", "err", err)
		}
		pts := make([]activity.HourPoint, 0, len(rows))
		for _, r := range rows {
			pts = append(pts, activity.HourPoint{Hour: int(r.HourLocal), ActiveUsers: r.ActiveUsers, TotalHits: r.TotalHits})
		}
		filled := activity.FillHours(pts)
		peak = peakHourLabel(filled)
		opt = activity.HourlyBarOption(filled)
	} else {
		rows, err := h.q(ctx).PresenceByDay(ctx, db.PresenceByDayParams{Tz: tz, FromAt: fromTS, ToAt: toTS})
		if err != nil {
			h.Log.Error("dev logs: by day", "err", err)
		}
		pts := make([]activity.DayPoint, 0, len(rows))
		for _, r := range rows {
			pts = append(pts, activity.DayPoint{Day: r.DayLocal.Time, ActiveUsers: r.ActiveUsers, TotalHits: r.TotalHits})
		}
		// Batas lokal untuk gap-fill tanggal.
		filled := activity.FillDays(pts, from.In(appTZ), to.In(appTZ))
		opt = activity.DailyTrendOption(filled)
	}
	b, err := json.Marshal(opt)
	if err != nil {
		h.Log.Error("dev logs: marshal chart", "err", err)
		return "{}", peak
	}
	return string(b), peak
}

// peakHourLabel mengembalikan jam tersibuk (mis. "14:00") berdasar total hits; ""
// bila semua nol.
func peakHourLabel(points []activity.HourPoint) string {
	best, hasData := activity.HourPoint{}, false
	for _, p := range points {
		if p.TotalHits > best.TotalHits {
			best, hasData = p, true
		}
	}
	if !hasData {
		return ""
	}
	return fmtHour(best.Hour)
}

// fmtHour memformat jam (0..23) jadi "HH:00".
func fmtHour(hr int) string {
	if hr < 10 {
		return "0" + strconv.Itoa(hr) + ":00"
	}
	return strconv.Itoa(hr) + ":00"
}

// buildSpans memetakan rentang aktivitas user (first/last seen diformat lokal).
func (h *Handler) buildSpans(ctx context.Context, fromTS, toTS pgtype.Timestamptz) []dev.SpanRow {
	rows, err := h.q(ctx).PresenceUserSpans(ctx, db.PresenceUserSpansParams{FromAt: fromTS, ToAt: toTS})
	if err != nil {
		h.Log.Error("dev logs: spans", "err", err)
		return nil
	}
	out := make([]dev.SpanRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, dev.SpanRow{
			Email:     r.Email,
			FirstSeen: fmtLocal(r.FirstSeen),
			LastSeen:  fmtLocal(r.LastSeen),
			Hits:      r.TotalHits,
		})
	}
	return out
}

// buildAuthEvents memetakan event login/logout terbaru.
func (h *Handler) buildAuthEvents(ctx context.Context) []dev.AuthRow {
	rows, err := h.q(ctx).ListAuthEvents(ctx, recentAuthLimit)
	if err != nil {
		h.Log.Error("dev logs: auth events", "err", err)
		return nil
	}
	out := make([]dev.AuthRow, 0, len(rows))
	for _, r := range rows {
		var uid int64
		if r.TargetID != nil {
			uid = *r.TargetID
		}
		out = append(out, dev.AuthRow{
			Action: r.Action,
			Method: authMethod(r.Metadata),
			UserID: uid,
			When:   fmtLocal(r.CreatedAt),
		})
	}
	return out
}

// authMethod mengekstrak {"method":...} dari metadata JSONB audit. "" bila absen.
func authMethod(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var m struct {
		Method string `json:"method"`
	}
	_ = json.Unmarshal(raw, &m)
	return m.Method
}

// fmtLocal memformat timestamptz ke waktu lokal aplikasi. "—" bila null.
func fmtLocal(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return "—"
	}
	return ts.Time.In(appTZ).Format("02 Jan 15:04")
}
