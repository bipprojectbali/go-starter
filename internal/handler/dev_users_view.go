package handler

import (
	"context"

	"go_starter/internal/db"
	"go_starter/internal/settings"
	"go_starter/internal/ui/pages/dev"
)

// dev_users_view.go — pemetaan baris DB → baris view panel /dev/users, plus
// query batch pendukungnya. Dipisah dari handler HTTP-nya: bentuk tampilan
// berubah mengikuti kebutuhan layar, bukan mengikuti aturan siapa boleh apa.

// toUserRows memetakan db.User + membership-nya → dev.UserRow (view). Role kini
// PER-WORKSPACE: tiap user membawa daftar workspace tempat ia jadi anggota, jadi
// panel platform bisa mengubah role di workspace mana pun. byUser = hasil
// ListMembershipsForUsers (satu query batch — hindari N+1, Rule 13).
func toUserRows(users []db.User, byUser map[int64][]dev.WorkspaceRole) []dev.UserRow {
	rows := make([]dev.UserRow, 0, len(users))
	for _, u := range users {
		avatar := ""
		if u.AvatarUrl != nil {
			avatar = *u.AvatarUrl
		}
		isRoot := isSuperAdminEmail(u.Email)
		rows = append(rows, dev.UserRow{
			ID:         u.ID,
			Email:      u.Email,
			Status:     u.Status,
			AvatarURL:  avatar,
			IsRoot:     isRoot,
			Workspaces: byUser[u.ID],
			// Kuota EFEKTIF + apakah itu hak khusus. Keduanya dioper terpisah agar
			// panel bisa menampilkan "3 (global)" vs "5 (khusus)": tanpa penanda,
			// operator tak bisa tahu mana yang akan ikut berubah saat default
			// global diubah — justru pertanyaan yang membuat halaman ini berguna.
			Quota:         settings.EffectiveWorkspaceQuota(u.WorkspaceQuota),
			QuotaOverride: u.WorkspaceQuota != nil,
		})
	}
	return rows
}

// membershipsByUser menjalankan SATU query batch untuk semua user lalu
// mengelompokkannya per user_id (hindari N+1). Error → map kosong (fail-soft:
// tabel tetap tampil, kolom workspace saja yang kosong).
func (h *Handler) membershipsByUser(ctx context.Context, users []db.User) map[int64][]dev.WorkspaceRole {
	ids := make([]int64, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	out := make(map[int64][]dev.WorkspaceRole, len(users))
	if len(ids) == 0 {
		return out
	}
	rows, err := h.q(ctx).ListMembershipsForUsers(ctx, ids)
	if err != nil {
		h.Log.Error("dev users: list memberships", "err", err)
		return out
	}
	for _, m := range rows {
		out[m.UserID] = append(out[m.UserID], dev.WorkspaceRole{
			TenantID: m.TenantID, Name: m.Name, Role: m.Role,
		})
	}
	return out
}
