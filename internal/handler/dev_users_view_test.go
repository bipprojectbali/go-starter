package handler

import (
	"testing"

	"go_starter/internal/db"
	"go_starter/internal/settings"
	"go_starter/internal/ui/pages/dev"
)

// TestToUserRows_MarksRootAndMemberships menjaga pemetaan user → baris panel /dev
// setelah model membership: users GLOBAL (tanpa kolom role), role dibawa
// per-workspace lewat memberships, dan super-admin env ditandai IsRoot.
func TestToUserRows_MarksRootAndMemberships(t *testing.T) {
	SetSuperAdminChecker(func(email string) bool { return email == "root@x.com" })
	t.Cleanup(func() { SetSuperAdminChecker(func(string) bool { return false }) })

	// workspace_quota kini NULLABLE: nil = ikut default global, angka = hak
	// khusus. Keduanya diwakili di sini agar pemetaan QuotaOverride ikut terjaga.
	khusus := int32(5)
	users := []db.User{
		{ID: 1, Email: "root@x.com"},                           // super-admin env, ikut global
		{ID: 2, Email: "biasa@x.com", WorkspaceQuota: &khusus}, // hak khusus
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
	// Kuota EFEKTIF + asalnya. Menampilkan angka saja tak cukup: operator perlu
	// tahu siapa yang akan ikut berubah saat default global diubah.
	if rows[1].Quota != 5 || !rows[1].QuotaOverride {
		t.Errorf("hak khusus harus terbawa apa adanya, got %d (override=%v)",
			rows[1].Quota, rows[1].QuotaOverride)
	}
	if rows[0].QuotaOverride {
		t.Error("user tanpa override tak boleh ditandai punya hak khusus")
	}
	// Tanpa override → angka datang dari default global, bukan dari user.
	if rows[0].Quota != settings.WorkspaceQuotaDefault() {
		t.Errorf("tanpa override harus ikut global %d, got %d",
			settings.WorkspaceQuotaDefault(), rows[0].Quota)
	}
}
