// Package panel berisi halaman umum panel (admin/user) yang berbagi AppShell.
package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Placeholder merender halaman stub sederhana (judul + deskripsi) untuk panel
// yang belum berisi konten. Dipakai /admin & /user sebagai landing awal.
func Placeholder(title, desc string) g.Node {
	return h.Div(
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text(title)),
		h.P(h.Class("text-base-content/70"), g.Text(desc)),
	)
}
