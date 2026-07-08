package ui

import (
	"strings"
	"testing"

	h "maragu.dev/gomponents/html"
)

// TestGoogleG_BrandCompliance menjaga kepatuhan merek Google: keempat warna
// resmi "super G" WAJIB hadir dan tak boleh diubah (syarat verifikasi OAuth).
// Test ini regresi — mencegah logo tak sengaja di-recolor/dihapus warnanya.
func TestGoogleG_BrandCompliance(t *testing.T) {
	var sb strings.Builder
	if err := GoogleG(h.Class("size-[18px]")).Render(&sb); err != nil {
		t.Fatalf("render GoogleG: %v", err)
	}
	out := sb.String()

	// Empat warna resmi Google — semua wajib ada.
	for _, color := range []string{"#EA4335", "#4285F4", "#FBBC05", "#34A853"} {
		if !strings.Contains(out, color) {
			t.Errorf("warna merek Google %s hilang dari logo:\n%s", color, out)
		}
	}
	// Aspect ratio resmi tak boleh berubah.
	if !strings.Contains(out, `viewBox="0 0 48 48"`) {
		t.Errorf("viewBox logo Google harus 0 0 48 48 (aspect ratio resmi):\n%s", out)
	}
	// Class ukuran diteruskan (ukuran diatur via class, bukan ubah viewBox).
	if !strings.Contains(out, "size-[18px]") {
		t.Errorf("atribut class tidak diteruskan ke svg:\n%s", out)
	}
	// aria-hidden: logo dekoratif (teks tombol yang menyuarakan aksinya).
	if !strings.Contains(out, `aria-hidden="true"`) {
		t.Errorf("logo dekoratif harus aria-hidden:\n%s", out)
	}
}
