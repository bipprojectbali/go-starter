package handler

import (
	"testing"

	"go_starter/internal/ui"
)

// panel_ident_test.go — identitas visual shell (chip + aksen). Salah petakan =
// user mengira sedang di panel lain; untuk /dev itu berarti salah membaca
// CAKUPAN DATA (panel platform menampilkan semua workspace).

// Sejak 0004 hanya /dev yang ditentukan PATH: satu alamat /w/{slug} melayani
// semua role, jadi chip-nya diturunkan dari role (panelForRole) — kalau tidak,
// member dan owner di halaman yang sama akan melihat chip identik padahal
// otoritas mereka berbeda.
func TestPanelFor_PathKeIdentitas(t *testing.T) {
	cases := map[string]ui.Panel{
		"/dev/users":      ui.PanelDev,
		"/dev/logs":       ui.PanelDev,
		"/dev":            ui.PanelDev,
		"/w/acme":         ui.PanelNone, // ruang kerja → ditentukan role
		"/w/acme/members": ui.PanelNone,
		"/notifications":  ui.PanelNone, // lintas-panel → ditentukan role
		"/workspace/new":  ui.PanelNone,
		"/":               ui.PanelNone,
	}
	for path, want := range cases {
		if got := panelFor(path); got != want {
			t.Errorf("panelFor(%q) = %v, want %v", path, got, want)
		}
	}
}

// TestPanelOf_LintasPanelIkutRole: chip /notifications HARUS sama dengan panel
// menu yang tampil (navFor). Kalau berbeda, user melihat menu /dev berlabel
// "RUANG KERJA" — lebih membingungkan daripada tanpa chip sama sekali.
func TestPanelOf_LintasPanelIkutRole(t *testing.T) {
	env, _ := setupTest(t)
	cases := map[string]ui.Panel{
		"member":      ui.PanelUser,
		"admin":       ui.PanelAdmin,
		"owner":       ui.PanelAdmin,
		"staff":       ui.PanelDev,
		"super_admin": ui.PanelDev,
	}
	for role, want := range cases {
		ctx := ctxWithRole(t, role)
		if got := env.h.panelOf(ctx, "/notifications"); got != want {
			t.Errorf("role %s di /notifications: panel %v, want %v", role, got, want)
		}
	}
}

// TestPanelOf_DevPathMenangAtasRole: /dev adalah SATU-SATUNYA path yang menang
// atas role — panel platform menampilkan data LINTAS-workspace, jadi chip
// PLATFORM harus muncul di sana apa pun role tenant yang dipegang user.
func TestPanelOf_DevPathMenangAtasRole(t *testing.T) {
	env, _ := setupTest(t)
	for _, role := range []string{"member", "admin", "owner"} {
		ctx := ctxWithRole(t, role)
		if got := env.h.panelOf(ctx, "/dev/users"); got != ui.PanelDev {
			t.Errorf("role %s di /dev/users: panel %v, want PanelDev", role, got)
		}
	}
}

// TestPanelOf_RuangKerjaIkutRole: di /w/{slug} chip menandakan OTORITAS user di
// workspace itu. Setelah peleburan 0004 alamatnya sama untuk semua, jadi chip
// inilah satu-satunya petunjuk bahwa owner & member melihat halaman yang sama
// dengan kewenangan berbeda.
func TestPanelOf_RuangKerjaIkutRole(t *testing.T) {
	env, _ := setupTest(t)
	cases := map[string]ui.Panel{
		"member":      ui.PanelUser,
		"admin":       ui.PanelAdmin,
		"owner":       ui.PanelAdmin,
		"super_admin": ui.PanelDev,
	}
	for role, want := range cases {
		ctx := ctxWithRole(t, role)
		if got := env.h.panelOf(ctx, "/w/acme/members"); got != want {
			t.Errorf("role %s di /w/acme/members: panel %v, want %v", role, got, want)
		}
	}
}

// TestPanelIdentitasUnik: ketiga panel wajib punya label DAN warna yang berbeda
// — kalau ada yang sama, pembedanya hilang justru di titik yang sedang kita
// perbaiki. Ini menjaga siapa pun yang menambah panel baru nanti.
func TestPanelIdentitasUnik(t *testing.T) {
	panels := []ui.Panel{ui.PanelUser, ui.PanelAdmin, ui.PanelDev}
	seenLabel := map[string]bool{}
	seenChip := map[string]bool{}
	for _, p := range panels {
		l, chip, edge := ui.PanelIdentity(p)
		if l == "" || chip == "" || edge == "" {
			t.Errorf("panel %v tak lengkap: label=%q chip=%q edge=%q", p, l, chip, edge)
			continue
		}
		if seenLabel[l] {
			t.Errorf("label %q dipakai dua panel — pembeda hilang", l)
		}
		if seenChip[chip] {
			t.Errorf("warna chip %q dipakai dua panel — pembeda hilang", chip)
		}
		seenLabel[l], seenChip[chip] = true, true
	}
	// PanelNone sengaja kosong (halaman di luar ketiga panel).
	if l, _, _ := ui.PanelIdentity(ui.PanelNone); l != "" {
		t.Errorf("PanelNone harus tanpa identitas, got %q", l)
	}
}
