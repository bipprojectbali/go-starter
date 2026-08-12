package ui

// shelluser.go — blok identitas user di dasar sidebar (avatar, email, tema,
// logout). Berdiri sendiri: satu-satunya bagian sidebar yang menyentuh sesi user
// dan pemicu modal logout.

import (
	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sidebarUser = blok identitas di bagian bawah sidebar (avatar, email, logout).
func sidebarUser(d ShellData) g.Node {
	return h.Div(
		h.Class("border-t border-base-300 p-3 flex flex-col gap-2"),
		h.Div(
			h.Class("flex items-center justify-between gap-2 min-w-0"),
			h.Div(
				h.Class("flex items-center gap-2 min-w-0"),
				Avatar(d.AvatarURL, "", d.UserEmail, 32),
				h.Span(h.Class("app-navlabel text-sm truncate"), g.Text(d.UserEmail)),
			),
			ThemeToggleUp(),
		),
		// Ganti akun = pindah akun tanpa harus logout dulu. Link GET ke flow OAuth
		// dgn ?switch=1 (pemilih akun Google). Bukan aksi destruktif: bila dibatalkan
		// di Google, sesi saat ini tetap utuh — jadi tanpa modal konfirmasi. Callback
		// OAuth memutar & menimpa identitas (session.Renew), sehingga akun tergantikan.
		h.A(
			h.Href("/api/auth/google?switch=1"),
			h.Class("btn btn-ghost btn-sm w-full"),
			g.Attr("title", "Ganti akun"),
			lucide.UsersRound(h.Class("size-4")),
			h.Span(h.Class("app-navlabel"), g.Text("Ganti akun")),
		),
		h.Button(
			h.Class("btn btn-outline btn-sm w-full"),
			g.Attr("title", "Keluar"),
			ConfirmTrigger("logoutConfirm"), // buka modal, bukan langsung logout
			lucide.LogOut(h.Class("size-4")),
			h.Span(h.Class("app-navlabel"), g.Text("Keluar")),
		),
	)
}
