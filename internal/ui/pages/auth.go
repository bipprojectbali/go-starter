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
	return authPage(authOpts{
		title: "Masuk", action: "/login", showPassword: showPassword,
		switchText: "Belum punya akun?", switchHref: "/register", switchLabel: "Daftar",
	})
}

// Register merender halaman pendaftaran (dev-only; route-nya tak ada di prod).
// showWorkspace: register mengumpulkan Nama Workspace (buat workspace baru).
func Register() g.Node {
	return authPage(authOpts{
		title: "Daftar", action: "/register", showPassword: true, showWorkspace: true,
		switchText: "Sudah punya akun?", switchHref: "/login", switchLabel: "Masuk",
	})
}

// authOpts = parameter authPage (struct agar tak jadi daftar argumen panjang).
type authOpts struct {
	title, action                       string
	showPassword, showWorkspace         bool
	switchText, switchHref, switchLabel string
}

// authPage adalah kerangka bersama login & register: tombol Google + (opsional)
// form password (+ field Nama Workspace bila showWorkspace).
func authPage(o authOpts) g.Node {
	card := []g.Node{googleButton()}
	if o.showPassword {
		card = append(card, passwordDivider(), passwordFields(o.action, o.title, o.showWorkspace))
	}

	body := []g.Node{
		h.H1(h.Class("text-xl font-semibold mb-4"), g.Text(o.title)),
		ui.Card(card...),
	}
	// Tautan alih login/daftar hanya relevan saat form password aktif.
	if o.showPassword {
		body = append(body, h.P(
			h.Class("mt-4"),
			g.Text(o.switchText+" "),
			h.A(h.Href(o.switchHref), g.Text(o.switchLabel)),
		))
	}

	signals := map[string]any{"email": "", "password": ""}
	if o.showWorkspace {
		signals["workspace"] = ""
	}
	return h.Div(
		data.Signals(signals),
		g.Group(body),
	)
}

// googleButton — tautan penuh (bukan @post) ke flow OAuth. Navigasi biasa 302.
// Logo "super G" 4-warna resmi + teks (lokalisasi diizinkan pedoman Google).
// daisyUI .btn sudah flex + gap, jadi logo & teks otomatis berjajar rapi.
func googleButton() g.Node {
	return h.A(
		h.Href("/api/auth/google"),
		h.Class("btn btn-outline w-full"),
		ui.GoogleG(h.Class("size-[18px]")), // 18px = spesifikasi Google
		g.Text("Masuk dengan Google"),
	)
}

// passwordDivider memberi pemisah visual "atau" antara Google dan form password.
func passwordDivider() g.Node {
	return h.P(h.Class("text-center text-sm text-base-content/70"), g.Text("atau"))
}

// passwordFields adalah form email/password (dev-only). showWorkspace menambahkan
// field "Nama Workspace" di paling atas (hanya di register — buat workspace baru).
func passwordFields(action, submitLabel string, showWorkspace bool) g.Node {
	fields := []g.Node{}
	if showWorkspace {
		fields = append(fields, h.Div(
			h.Class("grid gap-2"),
			ui.Label("Nama Workspace", h.For("workspace")),
			ui.Input(
				h.ID("workspace"),
				h.Type("text"),
				data.Bind("workspace"),
				h.Placeholder("mis. Acme Corp"),
			),
		))
	}
	fields = append(fields,
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
			[]g.Node{data.On("click", ui.PostAction(action))},
			g.Text(submitLabel),
		),
		// Slot error kosong — di-patch jadi Alert berisi saat auth gagal.
		ui.AlertSlot("auth-error"),
	)
	return g.Group(fields)
}
