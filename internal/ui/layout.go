// Package ui berisi komponen HTML sebagai fungsi Go (gomponents).
package ui

import (
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	c "maragu.dev/gomponents/components"
	h "maragu.dev/gomponents/html"
)

// LayoutData memuat konteks yang dibutuhkan kerangka halaman.
type LayoutData struct {
	Title     string // judul <title>
	UserEmail string // "" bila belum login (nav sembunyi)
	AvatarURL string // URL avatar Google (base); "" → inisial
	CSSPath   string // path app.css (dengan cache-bust hash)
}

// Layout membungkus konten halaman dengan HTML5 lengkap: head, CSS, nav, dan
// runtime Datastar (vendored). Dipakai untuk full-page render (§4.4 jalur 1).
func Layout(d LayoutData, body ...g.Node) g.Node {
	cssPath := d.CSSPath
	if cssPath == "" {
		cssPath = "/static/app.css"
	}
	return c.HTML5(c.HTML5Props{
		Title:    d.Title,
		Language: "id",
		Head:     headNodes(cssPath),
		Body: []g.Node{
			h.Class("min-h-screen bg-background text-foreground"),
			nav(d.UserEmail, d.AvatarURL),
			h.Main(h.Class("mx-auto max-w-2xl p-6"), g.Group(body)),
			// Modal konfirmasi logout — hanya relevan bila ada nav (user login).
			logoutModal(d.UserEmail),
		},
	})
}

// headNodes = isi <head> bersama (favicon, CSS, Datastar). Dipakai Layout &
// AppShell agar tak duplikat (DRY).
func headNodes(cssPath string) []g.Node {
	return []g.Node{
		h.Link(h.Rel("icon"), h.Type("image/svg+xml"), h.Href("/static/favicon.svg")),
		h.Link(h.Rel("stylesheet"), h.Href("/static/basecoat.css")),
		h.Link(h.Rel("stylesheet"), h.Href(cssPath)),
		h.Script(h.Src("/static/datastar.js"), h.Type("module"), h.Defer()),
	}
}

// nav menampilkan bar atas. Tombol logout + avatar muncul hanya bila user login.
func nav(userEmail, avatarURL string) g.Node {
	if userEmail == "" {
		return g.Text("") // belum login (mis. halaman login) — tanpa nav
	}
	return h.Header(
		h.Class("border-b"),
		data.Signals(map[string]any{"logoutConfirm": false}),
		h.Div(
			h.Class("mx-auto max-w-2xl p-4 flex items-center justify-between"),
			h.A(h.Href("/todos"), h.Class("font-semibold"), g.Text("go_stater")),
			h.Div(
				h.Class("flex items-center gap-3"),
				Avatar(avatarURL, "", userEmail, 32),
				h.Span(h.Class("text-sm"), g.Text(userEmail)),
				h.Button(
					h.Class("btn"),
					g.Attr("data-variant", "outline"),
					g.Attr("data-size", "sm"),
					ConfirmTrigger("logoutConfirm"), // buka modal, bukan langsung logout
					g.Text("Keluar"),
				),
			),
		),
	)
}

// logoutModal merender modal konfirmasi logout untuk Layout; kosong bila anonim.
func logoutModal(userEmail string) g.Node {
	if userEmail == "" {
		return g.Text("")
	}
	return ConfirmModal("logoutConfirm", "Keluar?",
		"Anda akan keluar dari sesi ini.", "Keluar", "@post('/logout')")
}
