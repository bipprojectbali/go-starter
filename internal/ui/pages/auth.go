package pages

import (
	"go_stater/internal/ui"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// Login merender halaman masuk. Tombol Google selalu tampil (jalur utama);
// form email/password hanya bila showPassword (dev — password auth dev-only).
func Login(showPassword bool) g.Node {
	return authPage("Masuk", "/login", showPassword, "Belum punya akun?", "/register", "Daftar")
}

// Register merender halaman pendaftaran (dev-only; route-nya tak ada di prod).
func Register() g.Node {
	return authPage("Daftar", "/register", true, "Sudah punya akun?", "/login", "Masuk")
}

// authPage adalah kerangka bersama login & register: tombol Google + (opsional)
// form password. action = endpoint POST form password.
func authPage(title, action string, showPassword bool, switchText, switchHref, switchLabel string) g.Node {
	card := []g.Node{googleButton()}
	if showPassword {
		card = append(card, passwordDivider(), passwordFields(action, title))
	}

	body := []g.Node{
		h.H1(h.Class("text-xl font-semibold mb-4"), g.Text(title)),
		ui.Card(card...),
	}
	// Tautan alih login/daftar hanya relevan saat form password aktif.
	if showPassword {
		body = append(body, h.P(
			h.Class("mt-4"),
			g.Text(switchText+" "),
			h.A(h.Href(switchHref), g.Text(switchLabel)),
		))
	}

	return h.Div(
		data.Signals(map[string]any{"email": "", "password": ""}),
		g.Group(body),
	)
}

// googleButton — tautan penuh (bukan @post) ke flow OAuth. Navigasi biasa 302.
// Logo "super G" 4-warna resmi + teks (lokalisasi diizinkan pedoman Google).
// Basecoat .btn sudah flex + gap, jadi logo & teks otomatis berjajar rapi.
func googleButton() g.Node {
	return h.A(
		h.Href("/api/auth/google"),
		h.Class("btn w-full"),
		g.Attr("data-variant", "outline"),
		ui.GoogleG(h.Class("size-[18px]")), // 18px = spesifikasi Google
		g.Text("Masuk dengan Google"),
	)
}

// passwordDivider memberi pemisah visual "atau" antara Google dan form password.
func passwordDivider() g.Node {
	return h.P(h.Class("text-center text-sm text-muted-foreground"), g.Text("atau"))
}

// passwordFields adalah form email/password (dev-only).
func passwordFields(action, submitLabel string) g.Node {
	return g.Group([]g.Node{
		h.Div(
			h.Class("grid gap-2"),
			ui.Label("Email", h.For("email")),
			ui.Input(
				h.ID("email"),
				h.Type("email"),
				data.Bind("email"),
				h.Placeholder("nama@contoh.com"),
			),
		),
		h.Div(
			h.Class("grid gap-2"),
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
			g.Text(submitLabel),
		),
		// Slot error kosong — di-patch jadi Alert berisi saat auth gagal.
		ui.AlertSlot("auth-error"),
	})
}
