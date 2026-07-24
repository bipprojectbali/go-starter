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
	out := renderNode(t, ConfirmModal("logoutConfirm", "Keluar?", "Yakin?", "Keluar", "/logout"))

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
	// Tombol konfirmasi = NATIVE form POST (bukan @post — redirect SSE diblokir
	// CSP). Assert form method+action, BUKAN ekspresi Datastar.
	for _, want := range []string{`method="post"`, `action="/logout"`, `type="submit"`} {
		if !strings.Contains(out, want) {
			t.Errorf("tombol konfirmasi harus native form submit, kurang %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "@post") {
		t.Errorf("ConfirmModal TAK boleh pakai @post (navigasi = native form):\n%s", out)
	}
	// Batal & backdrop menutup via signal.
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
