package handler

import "testing"

// TestDevNav_FileHealthDevOnly menjaga agar menu "File Health" HANYA muncul di
// dev — route-nya tak terdaftar di produksi (source .go tak ada di single-binary).
func TestDevNav_FileHealthDevOnly(t *testing.T) {
	hasFileHealth := func() bool {
		for _, it := range devNav() {
			if it.Href == "/dev/health" {
				return true
			}
		}
		return false
	}

	SetDevMode(true)
	if !hasFileHealth() {
		t.Error("di dev, menu File Health harus ada")
	}
	// Users selalu ada.
	if len(devNav()) < 1 || devNav()[0].Href != "/dev/users" {
		t.Error("menu Users harus selalu ada")
	}

	SetDevMode(false)
	if hasFileHealth() {
		t.Error("di produksi, menu File Health TIDAK boleh ada")
	}
	SetDevMode(true) // pulihkan default test
}
