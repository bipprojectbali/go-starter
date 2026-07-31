package handler

import (
	"net/http"
	"time"

	"go_starter/internal/activity"
	"go_starter/internal/db"
	"go_starter/internal/ui/pages/dev"

	"github.com/jackc/pgx/v5/pgtype"
)

// DevLogs — GET /dev/logs?range=day|week|month. Dashboard aktivitas user:
// presence (jam berapa aktif), KPI, chart, event login/logout. Owner-only.
// Builder view-model (buildChart/buildSpans/buildAuthEvents) di dev_logs_build.go.
func (h *Handler) DevLogs(w http.ResponseWriter, r *http.Request) {
	rng := activity.ParseRange(r.URL.Query().Get("range"))
	from, to := rng.Window(time.Now(), appTZ)
	fromTS := pgtype.Timestamptz{Time: from, Valid: true}
	toTS := pgtype.Timestamptz{Time: to, Valid: true}
	ctx := r.Context()

	vm := dev.LogsData{
		Range:      string(rng),
		RangeLabel: rng.Label(),
		IsDaily:    rng.IsDaily(),
	}
	if kpi, err := h.q(ctx).PresenceKPIs(ctx, db.PresenceKPIsParams{FromAt: fromTS, ToAt: toTS}); err != nil {
		h.Log.Error("dev logs: kpi", "err", err)
	} else {
		vm.ActiveUsers, vm.TotalHits = kpi.ActiveUsers, kpi.TotalHits
	}
	vm.ChartJSON, vm.PeakHour = h.buildChart(ctx, rng, fromTS, toTS, from, to)
	vm.Spans = h.buildSpans(ctx, fromTS, toTS)

	// Jejak aktivitas — MENGHORMATI rentang yang dipilih, tak seperti tabel
	// login/logout yang digantikannya: dulu ia selalu 20 terbaru, jadi memilih
	// "Bulanan" tak mengubah apa pun dan tab-nya berbohong.
	rows, next := h.buildTrail(ctx, r, fromTS, toTS)
	vm.Trail = trailViewOf(r, string(rng), rows, next,
		h.buildTrailFamilies(ctx, fromTS, toTS),
		h.buildTrailActors(ctx, fromTS, toTS))

	h.renderShell(w, r, "User Activity", devBrand(), "/dev/logs", devNav(r.Context()),
		dev.LogsPage(vm))
}
