package pages

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Landing merender halaman depan publik (route "/"). Dapat diakses SEMUA
// (tak redirect). CTA menyesuaikan status login: user login → tombol ke home
// per-role (homePath), anonim → "Masuk". homePath diabaikan bila anonim.
func Landing(loggedIn bool, homePath string) g.Node {
	return h.Div(
		h.Class("text-center py-16"),
		h.H1(
			h.Class("text-4xl font-bold tracking-tight mb-4"),
			g.Text("go_starter"),
		),
		h.P(
			h.Class("text-lg text-base-content/70 mb-8 max-w-md mx-auto"),
			g.Text("Starter full-stack Go: cepat, ringan, single binary. Datastar + gomponents + Postgres."),
		),
		landingCTA(loggedIn, homePath),
	)
}

// landingCTA memilih tombol aksi sesuai status login.
func landingCTA(loggedIn bool, homePath string) g.Node {
	if loggedIn {
		return h.Div(
			h.Class("flex items-center justify-center gap-3"),
			h.A(h.Href(homePath), h.Class("btn btn-primary"), g.Text("Buka aplikasi")),
		)
	}
	// Anonim: arahkan ke /login (adaptif — Google + form password bila dev).
	return h.Div(
		h.Class("flex items-center justify-center gap-3"),
		h.A(h.Href("/login"), h.Class("btn btn-primary"), g.Text("Masuk")),
	)
}
