package ui

import (
	"strings"
	"testing"

	g "maragu.dev/gomponents"
)

// panelkind_test.go — penanda panel di shell. Menjaga DUA penanda tetap ada:
// chip teks (terbaca bila warna tak tertangkap mata) DAN aksen tepi (tertangkap
// tanpa membaca, dan satu-satunya yang bertahan saat sidebar jadi rail ikon —
// `.app-brand` disembunyikan penuh oleh input.css saat collapsed).

func shellWithPanel(p Panel, workspace string) string {
	d := ShellData{
		Title:         "T",
		BrandLabel:    "go_starter /dev",
		WorkspaceName: workspace,
		CurrentPath:   "/x",
		Panel:         p,
		Nav:           []NavItem{{Label: "A", Href: "/x"}},
	}
	var sb strings.Builder
	AppShell(d, g.Text("konten")).Render(&sb)
	return sb.String()
}

// TestPanelChipDanAksenIkutRender: kedua penanda wajib muncul — kehilangan salah
// satunya menghapus pembeda untuk sebagian orang atau sebagian keadaan.
func TestPanelChipDanAksenIkutRender(t *testing.T) {
	cases := []struct{ panel Panel }{{PanelUser}, {PanelAdmin}, {PanelDev}}
	for _, c := range cases {
		out := shellWithPanel(c.panel, "Acme")
		label, chip, edge := PanelIdentity(c.panel)
		if !strings.Contains(out, label) {
			t.Errorf("panel %v: chip teks %q tak dirender", c.panel, label)
		}
		if !strings.Contains(out, chip) {
			t.Errorf("panel %v: class chip %q tak dirender", c.panel, chip)
		}
		if !strings.Contains(out, edge) {
			t.Errorf("panel %v: aksen tepi %q tak dirender — penanda hilang saat rail collapsed", c.panel, edge)
		}
	}
}

// TestPanelChipTampilTanpaWorkspace: platform sering tanpa konteks workspace
// (WorkspaceName kosong) — justru di situ chip PLATFORM paling dibutuhkan,
// karena cabang render brand-nya berbeda dan mudah terlewat.
func TestPanelChipTampilTanpaWorkspace(t *testing.T) {
	out := shellWithPanel(PanelDev, "")
	if !strings.Contains(out, "PLATFORM") {
		t.Error("chip harus tetap tampil saat WorkspaceName kosong (cabang brand berbeda)")
	}
	if !strings.Contains(out, "bg-warning") {
		t.Error("aksen tepi harus tetap tampil saat WorkspaceName kosong")
	}
}

// TestPanelNoneTanpaPenanda: halaman di luar ketiga panel tak boleh mengklaim
// identitas panel mana pun — chip palsu lebih menyesatkan daripada tak ada chip.
func TestPanelNoneTanpaPenanda(t *testing.T) {
	out := shellWithPanel(PanelNone, "Acme")
	for _, s := range []string{"RUANG KERJA", "ADMIN", "PLATFORM", "badge-warning", "badge-secondary"} {
		if strings.Contains(out, s) {
			t.Errorf("PanelNone tak boleh merender penanda panel, tapi memuat %q", s)
		}
	}
	// Fallback: sub-label lama tetap tampil agar baris kedua tak kosong.
	if !strings.Contains(out, "go_starter /dev") {
		t.Error("tanpa panel, sub-label BrandLabel harus jadi fallback")
	}
}
