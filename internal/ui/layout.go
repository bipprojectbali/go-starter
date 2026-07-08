// Package ui berisi komponen HTML sebagai fungsi Go (gomponents).
package ui

import (
	g "maragu.dev/gomponents"
	c "maragu.dev/gomponents/components"
	h "maragu.dev/gomponents/html"
)

// Layout membungkus konten halaman dengan HTML5 lengkap: head, CSS, dan
// runtime Datastar (vendored). Dipakai untuk full-page render (§4.4 jalur 1).
func Layout(title string, body ...g.Node) g.Node {
	return c.HTML5(c.HTML5Props{
		Title:    title,
		Language: "id",
		Head: []g.Node{
			h.Link(h.Rel("stylesheet"), h.Href("/static/app.css")),
			// Datastar runtime — vendored lokal, bukan CDN (§13).
			h.Script(h.Src("/static/datastar.js"), h.Type("module"), h.Defer()),
		},
		Body: []g.Node{
			h.Class("min-h-screen bg-background text-foreground"),
			h.Div(h.Class("mx-auto max-w-2xl p-6"), g.Group(body)),
		},
	})
}
