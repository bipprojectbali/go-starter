package pages

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Landing merender halaman depan publik (route "/"). CTA menyesuaikan status
// login: user login diarahkan ke aplikasi, anonim ke daftar/masuk.
func Landing(loggedIn bool) g.Node {
	return h.Div(
		h.Class("text-center py-16"),
		h.H1(
			h.Class("text-4xl font-bold tracking-tight mb-4"),
			g.Text("go_stater"),
		),
		h.P(
			h.Class("text-lg text-muted-foreground mb-8 max-w-md mx-auto"),
			g.Text("Starter full-stack Go: cepat, ringan, single binary. Datastar + gomponents + Postgres."),
		),
		landingCTA(loggedIn),
	)
}

// landingCTA memilih tombol aksi sesuai status login.
func landingCTA(loggedIn bool) g.Node {
	if loggedIn {
		return h.Div(
			h.Class("flex items-center justify-center gap-3"),
			h.A(h.Href("/todos"), h.Class("btn"), g.Text("Buka Todos")),
		)
	}
	return h.Div(
		h.Class("flex items-center justify-center gap-3"),
		h.A(h.Href("/register"), h.Class("btn"), g.Text("Mulai")),
		h.A(
			h.Href("/login"),
			h.Class("btn"),
			g.Attr("data-variant", "outline"),
			g.Text("Masuk"),
		),
	)
}
