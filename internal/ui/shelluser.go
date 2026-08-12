package ui

// shelluser.go — blok identitas user di dasar sidebar (avatar, email, tema,
// logout). Berdiri sendiri: satu-satunya bagian sidebar yang menyentuh sesi user
// dan pemicu modal logout.

import (
	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sidebarUser = blok identitas di dasar sidebar. Avatar+email = PEMICU dropdown
// akun (Ganti akun / Keluar); Tema tetap di sampingnya (preferensi tampilan,
// bukan aksi akun). Dulu tiga tombol bertumpuk — digabung jadi satu menu yang
// dibuka saat user diklik.
func sidebarUser(d ShellData) g.Node {
	return h.Div(
		h.Class("border-t border-base-300 p-3 flex items-center justify-between gap-2 min-w-0"),
		userMenu(d.AvatarURL, d.UserEmail, true),
		ThemeToggleUp(),
	)
}

// userMenu = dropdown identitas: avatar + email sebagai pemicu, isinya aksi akun
// "Ganti akun" dan "Keluar". Satu titik masuk saat user diklik (menggantikan dua
// tombol terpisah). Pola <details> CSS-only sama seperti workspaceSwitcher &
// themeDropdown — CSP-safe, tanpa inline JS.
//
//   - Ganti akun → /account/switch (dijaga RequireAuth: pintu ganti akun untuk
//     yang SUDAH masuk; /api/auth/google dijaga RequireGuest jadi tak bisa dipakai
//     di sini). Bukan aksi destruktif — batal di Google = sesi tetap utuh.
//   - Keluar → ConfirmTrigger membuka modal logoutConfirm (native POST /logout,
//     303). Modalnya dirender sekali oleh pemanggil (AppShell / logoutModal).
//
// openUp=true untuk footer sidebar (buka ke ATAS agar tak terpotong tepi bawah);
// false untuk header (buka ke bawah, rata kanan).
func userMenu(avatarURL, email string, openUp bool) g.Node {
	ddCls := "dropdown dropdown-end"
	menuCls := "dropdown-content menu bg-base-100 border border-base-300 rounded-box z-50 mt-2 w-56 p-2 shadow-lg"
	if openUp {
		ddCls = "dropdown dropdown-top flex-1 min-w-0"
		menuCls = "dropdown-content menu bg-base-100 border border-base-300 rounded-box z-50 mb-2 w-56 p-2 shadow-lg"
	}
	return h.Details(
		h.Class(ddCls),
		h.Summary(
			h.Class("btn btn-ghost btn-sm justify-between gap-1 px-1 h-auto py-1 min-w-0"),
			g.Attr("aria-label", "Menu akun"),
			h.Div(
				h.Class("flex items-center gap-2 min-w-0"),
				Avatar(avatarURL, "", email, 32),
				h.Span(h.Class("app-navlabel text-sm truncate"), g.Text(email)),
			),
			lucide.ChevronsUpDown(h.Class("app-navlabel size-4 shrink-0 opacity-60")),
		),
		h.Ul(
			h.Class(menuCls),
			h.Li(h.A(
				h.Href("/account/switch"), h.Class("gap-2"),
				g.Attr("title", "Ganti akun"),
				lucide.UsersRound(h.Class("size-4")),
				g.Text("Ganti akun"),
			)),
			h.Li(
				h.Class("border-t border-base-300 mt-1 pt-1"),
				h.Button(
					h.Type("button"), h.Class("gap-2 text-error"),
					g.Attr("title", "Keluar"),
					ConfirmTrigger("logoutConfirm"), // buka modal, bukan langsung logout
					lucide.LogOut(h.Class("size-4")),
					g.Text("Keluar"),
				),
			),
		),
	)
}
