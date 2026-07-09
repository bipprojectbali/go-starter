package ui

import (
	"strings"
	"testing"

	g "maragu.dev/gomponents"
)

func renderNode(t *testing.T, n g.Node) string {
	t.Helper()
	var sb strings.Builder
	if err := n.Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

func TestConfirmModal(t *testing.T) {
	out := renderNode(t, ConfirmModal("logoutConfirm", "Keluar?", "Yakin?", "Keluar", "@post('/logout')"))

	// Tampil hanya saat signal true.
	if !strings.Contains(out, `data-show="$logoutConfirm"`) {
		t.Errorf("modal harus data-show pada signal:\n%s", out)
	}
	// Judul + pesan + tombol.
	for _, want := range []string{"Keluar?", "Yakin?", ">Batal<"} {
		if !strings.Contains(out, want) {
			t.Errorf("modal kurang %q:\n%s", want, out)
		}
	}
	// Tombol Ya: jalankan aksi LALU tutup.
	if !strings.Contains(out, "@post(&#39;/logout&#39;); $logoutConfirm = false") {
		t.Errorf("tombol konfirmasi harus jalankan aksi lalu tutup:\n%s", out)
	}
	// Batal & backdrop menutup.
	if !strings.Contains(out, "$logoutConfirm = false") {
		t.Errorf("harus ada aksi tutup:\n%s", out)
	}
}

func TestConfirmTrigger(t *testing.T) {
	out := renderNode(t, ConfirmTrigger("logoutConfirm"))
	if !strings.Contains(out, "$logoutConfirm = true") {
		t.Errorf("trigger harus set signal true:\n%s", out)
	}
	// Trigger TIDAK boleh langsung @post (itu poin konfirmasi).
	if strings.Contains(out, "@post") {
		t.Errorf("trigger tak boleh langsung @post:\n%s", out)
	}
}
