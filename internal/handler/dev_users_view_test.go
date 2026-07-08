package handler

import (
	"testing"

	"go_stater/internal/db"
)

// TestToUserRows_EffectiveRoleForRoot menjaga agar super-admin env (root)
// ditampilkan dengan role EFEKTIF (super_admin), bukan kolom DB mentah (yang
// bisa "user" karena role env tak pernah ditulis ke DB). Tanpa ini, tabel
// menyesatkan: root tampak "user".
func TestToUserRows_EffectiveRoleForRoot(t *testing.T) {
	SetSuperAdminChecker(func(email string) bool { return email == "root@x.com" })
	t.Cleanup(func() { SetSuperAdminChecker(func(string) bool { return false }) })

	users := []db.User{
		{ID: 1, Email: "root@x.com", Role: "user"},  // env root, kolom DB masih user
		{ID: 2, Email: "biasa@x.com", Role: "user"}, // user biasa
		{ID: 3, Email: "adm@x.com", Role: "admin"},  // admin DB
	}
	rows := toUserRows(users)

	if rows[0].Role != "super_admin" || !rows[0].IsRoot {
		t.Errorf("root env harus tampil role=super_admin + IsRoot, got role=%q IsRoot=%v",
			rows[0].Role, rows[0].IsRoot)
	}
	if rows[1].Role != "user" || rows[1].IsRoot {
		t.Errorf("user biasa harus role=user, bukan root: got role=%q IsRoot=%v",
			rows[1].Role, rows[1].IsRoot)
	}
	if rows[2].Role != "admin" {
		t.Errorf("admin DB harus tampil apa adanya, got %q", rows[2].Role)
	}
}
