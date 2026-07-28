package handler

import (
	"testing"

	"go_starter/internal/ui"
)

// panel_ident_test.go — identitas visual shell (chip + aksen). Salah petakan =
// user mengira sedang di panel lain; untuk /dev itu berarti salah membaca
// CAKUPAN DATA (panel platform menampilkan semua workspace).

func TestPanelFor_PathKeIdentitas(t *testing.T) {
	cases := map[string]ui.Panel{
		"/dev/users":        ui.PanelDev,
		"/dev/logs":         ui.PanelDev,
		"/admin":            ui.PanelAdmin,
		"/admin/members":    ui.PanelAdmin,
		"/user":             ui.PanelUser,
		"/notifications":    ui.PanelNone, // lintas-panel → ditentukan role
		"/workspace/new":    ui.PanelNone,
		"/":                 ui.PanelNone,
		"/dev":              ui.PanelDev,
		"/administrasi":     ui.PanelAdmin, // prefix — didokumentasikan, bukan kejutan
		"/userland/sesuatu": ui.PanelUser,
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

// TestPanelOf_PathMenangAtasRole: di dalam panel yang jelas, path yang menentukan
// — super_admin membuka /user harus melihat identitas RUANG KERJA, bukan PLATFORM.
func TestPanelOf_PathMenangAtasRole(t *testing.T) {
	env, _ := setupTest(t)
	ctx := ctxWithRole(t, "super_admin")
	if got := env.h.panelOf(ctx, "/user"); got != ui.PanelUser {
		t.Errorf("super_admin di /user: panel %v, want PanelUser", got)
	}
	if got := env.h.panelOf(ctx, "/admin"); got != ui.PanelAdmin {
		t.Errorf("super_admin di /admin: panel %v, want PanelAdmin", got)
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
