package pages

import (
	"go_stater/internal/ui"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// Login merender halaman login dengan form Datastar.
func Login() g.Node {
	return authForm("Masuk", "/login", "Belum punya akun?", "/register", "Daftar")
}

// Register merender halaman pendaftaran.
func Register() g.Node {
	return authForm("Daftar", "/register", "Sudah punya akun?", "/login", "Masuk")
}

// authForm adalah kerangka bersama login & register. action = endpoint POST.
func authForm(title, action, switchText, switchHref, switchLabel string) g.Node {
	return h.Div(
		data.Signals(map[string]any{"email": "", "password": ""}),
		h.H1(h.Class("text-xl font-semibold mb-4"), g.Text(title)),
		ui.Card(
			h.Div(
				h.Class("mb-4"),
				ui.Label("Email", h.For("email")),
				ui.Input(
					h.ID("email"),
					h.Type("email"),
					data.Bind("email"),
					h.Placeholder("nama@contoh.com"),
				),
			),
			h.Div(
				h.Class("mb-4"),
				ui.Label("Password", h.For("password")),
				ui.Input(
					h.ID("password"),
					h.Type("password"),
					data.Bind("password"),
					h.Placeholder("••••••••"),
				),
			),
			ui.Button(
				ui.VariantDefault,
				[]g.Node{data.On("click", "@post('"+action+"')")},
				g.Text(title),
			),
			// Slot error — di-patch server saat auth gagal (butuh id, mode outer).
			ui.Alert(ui.VariantDestructive, "auth-error"),
		),
		h.P(
			h.Class("mt-4"),
			g.Text(switchText+" "),
			h.A(h.Href(switchHref), g.Text(switchLabel)),
		),
	)
}
