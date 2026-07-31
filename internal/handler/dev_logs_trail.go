package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"go_starter/internal/activity"
	"go_starter/internal/db"
	"go_starter/internal/ui/pages/dev"

	"github.com/jackc/pgx/v5/pgtype"
)

// dev_logs_trail.go — jejak aktivitas (audit_logs) untuk panel /dev/logs.
// Dipisah dari dev_logs_build.go yang menangani PRESENCE: keduanya menjawab
// pertanyaan berbeda (kapan orang ada vs siapa melakukan apa) dan tumbuh
// mengikuti kebutuhan yang berbeda pula. Penafsiran filternya di
// dev_logs_filter.go.

// buildTrail merakit tabel jejak aktivitas: baris + cursor halaman berikutnya.
func (h *Handler) buildTrail(ctx context.Context, r *http.Request, fromTS, toTS pgtype.Timestamptz) ([]dev.TrailRow, string) {
	f := parseTrailFilter(r)
	cursorAt, cursorID := pageCursor(r)

	params := db.ListActivityTrailParams{
		FromAt: fromTS, ToAt: toTS,
		CursorCreatedAt: cursorAt, CursorID: cursorID,
		PageSize: pageSize + 1, // +1 = penanda "masih ada" (lihat splitPage)
	}
	if f.family != "" {
		params.ActionPrefix = &f.family
	}
	if f.actorID != 0 {
		params.ActorID = &f.actorID
	}

	rows, err := h.q(ctx).ListActivityTrail(ctx, params)
	if err != nil {
		h.Log.Error("dev logs: trail", "err", err)
		return nil, ""
	}
	rows, next := splitPage(rows, func(a db.ListActivityTrailRow) (pgtype.Timestamptz, int64) {
		return a.CreatedAt, a.ID
	})

	out := make([]dev.TrailRow, 0, len(rows))
	for _, a := range rows {
		out = append(out, dev.TrailRow{
			Sentence: activity.Sentence(trailEvent(a)),
			Family:   activity.Family(a.Action),
			Action:   a.Action,
			When:     fmtLocal(a.CreatedAt),
		})
	}
	return out, next
}

// trailEvent memetakan baris SQL → Event. Nama diambil dari hasil JOIN dengan
// email sebagai cadangan: nama boleh kosong (akun password dev, atau Google tak
// mengirimkannya), dan baris tanpa penanda orang sama sekali tak bisa dibaca.
func trailEvent(a db.ListActivityTrailRow) activity.Event {
	var m struct {
		To     string `json:"to"`
		Role   string `json:"role"`
		Value  string `json:"value"`
		Reason string `json:"reason"`
		Method string `json:"method"`
	}
	if len(a.Metadata) > 0 {
		// Metadata rusak → kalimat tanpa detail, BUKAN baris yang hilang. Jejak
		// dibaca justru saat ada yang aneh; menyembunyikan barisnya karena satu
		// field tak terbaca menghilangkan bukti yang sedang dicari.
		_ = json.Unmarshal(a.Metadata, &m)
	}
	role := m.To
	if role == "" {
		role = m.Role
	}
	return activity.Event{
		Action:     a.Action,
		TargetType: a.TargetType,
		Actor:      identOf(a.ActorName, a.ActorEmail),
		TargetUser: identOf(a.TargetUserName, a.TargetUserEmail),
		Workspace:  deref(a.TargetWorkspaceName),
		Role:       role,
		Value:      m.Value,
		Reason:     m.Reason,
		Method:     m.Method,
	}
}

// buildTrailActors mengisi dropdown "filter per-orang" dari orang yang PUNYA
// jejak di rentang ini (fail-soft: gagal → dropdown kosong, tabel tetap tampil).
func (h *Handler) buildTrailActors(ctx context.Context, fromTS, toTS pgtype.Timestamptz) []dev.ActorOption {
	rows, err := h.q(ctx).ListActivityActors(ctx, db.ListActivityActorsParams{FromAt: fromTS, ToAt: toTS})
	if err != nil {
		h.Log.Error("dev logs: trail actors", "err", err)
		return nil
	}
	out := make([]dev.ActorOption, 0, len(rows))
	for _, a := range rows {
		// INNER JOIN users di query ini → email selalu ada (bukan pointer).
		out = append(out, dev.ActorOption{
			ID:     a.ActorID,
			Label:  identOf(a.ActorName, &a.ActorEmail),
			Events: a.Events,
		})
	}
	return out
}

// buildTrailFamilies mengisi filter jenis aksi, DENGAN jumlahnya: tanpa angka,
// operator harus mengklik satu per satu untuk menemukan yang berisi.
func (h *Handler) buildTrailFamilies(ctx context.Context, fromTS, toTS pgtype.Timestamptz) []dev.FamilyOption {
	rows, err := h.q(ctx).CountActivityByAction(ctx, db.CountActivityByActionParams{FromAt: fromTS, ToAt: toTS})
	if err != nil {
		h.Log.Error("dev logs: trail families", "err", err)
		return nil
	}
	out := make([]dev.FamilyOption, 0, len(rows))
	for _, r := range rows {
		if r.Family == "" {
			continue
		}
		out = append(out, dev.FamilyOption{Key: r.Family, Label: familyLabel(r.Family), Events: r.Events})
	}
	return out
}
