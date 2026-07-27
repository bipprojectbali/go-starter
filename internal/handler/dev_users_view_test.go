package handler

import (
	"testing"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/dev"
)

// TestToUserRows_MarksRootAndMemberships menjaga pemetaan user → baris panel /dev
// setelah model membership: users GLOBAL (tanpa kolom role), role dibawa
// per-workspace lewat memberships, dan super-admin env ditandai IsRoot.
func TestToUserRows_MarksRootAndMemberships(t *testing.T) {
	SetSuperAdminChecker(func(email string) bool { return email == "root@x.com" })
	t.Cleanup(func() { SetSuperAdminChecker(func(string) bool { return false }) })

	users := []db.User{
		{ID: 1, Email: "root@x.com", WorkspaceQuota: 3},  // super-admin env
		{ID: 2, Email: "biasa@x.com", WorkspaceQuota: 1}, // user biasa
	}
	byUser := map[int64][]dev.WorkspaceRole{
		2: {{TenantID: 9, Name: "Acme", Role: "owner"}},
	}
	rows := toUserRows(users, byUser)

	if !rows[0].IsRoot {
		t.Error("super-admin env harus ditandai IsRoot")
	}
	if rows[1].IsRoot {
		t.Error("user biasa tak boleh IsRoot")
	}
	// Keanggotaan ikut terbawa (role per-workspace, bukan properti user).
	if len(rows[1].Workspaces) != 1 || rows[1].Workspaces[0].Role != "owner" {
		t.Errorf("membership harus terbawa ke baris, got %+v", rows[1].Workspaces)
	}
	if len(rows[0].Workspaces) != 0 {
		t.Errorf("user tanpa membership harus punya daftar kosong, got %+v", rows[0].Workspaces)
	}
	// Kuota ikut ditampilkan (dasar kontrol platform).
	if rows[1].Quota != 1 {
		t.Errorf("kuota harus terbawa, got %d", rows[1].Quota)
	}
}
