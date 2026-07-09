package ui

import (
	"strings"
	"testing"
)

// TestThemeToggle_RendersAllThemesAsControllers memastikan tiap tema di
// themeList jadi radio ber-class `theme-controller` dengan value yang benar —
// itulah mekanisme daisyUI menerapkan tema (CSS-only).
func TestThemeToggle_RendersAllThemesAsControllers(t *testing.T) {
	out := renderNode(t, ThemeToggle())

	if !strings.Contains(out, "theme-controller") {
		t.Fatalf("harus ada input.theme-controller:\n%s", out)
	}
	// Radio saling eksklusif via name grup sama.
	if !strings.Contains(out, `name="theme-choice"`) {
		t.Errorf("radio tema harus satu grup name:\n%s", out)
	}
	// Semua tema terdaftar muncul sebagai value radio.
	for _, th := range themeList {
		if !strings.Contains(out, `value="`+th.Value+`"`) {
			t.Errorf("tema %q harus jadi opsi radio:\n%s", th.Value, out)
		}
	}
	// Hook theme.js untuk menutup dropdown setelah memilih.
	if !strings.Contains(out, "data-theme-dropdown") {
		t.Errorf("dropdown harus punya hook data-theme-dropdown:\n%s", out)
	}
}

// TestThemeToggleUp_OpensUpward: varian footer sidebar buka ke atas (dropdown-top)
// agar tak terpotong di tepi bawah layar.
func TestThemeToggleUp_OpensUpward(t *testing.T) {
	out := renderNode(t, ThemeToggleUp())
	if !strings.Contains(out, "dropdown-top") {
		t.Errorf("ThemeToggleUp harus dropdown-top:\n%s", out)
	}
	// Tetap membawa kontrol tema (bukan cuma arah beda).
	if !strings.Contains(out, "theme-controller") {
		t.Errorf("ThemeToggleUp harus tetap punya theme-controller:\n%s", out)
	}
}
