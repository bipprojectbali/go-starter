package pages

import (
	"go_stater/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Login merender halaman masuk. Tombol Google selalu tampil (jalur utama);
// form email/password hanya bila showPassword (dev — password auth dev-only).
// errMsg (dari ?err=) dirender sebagai alert di atas form (pola PRG — form native
// POST→303, bukan SSE; redirect via <script> diblokir CSP, lihat gotcha).
func Login(showPassword bool, errMsg string) g.Node {
	return authPage(authOpts{
		title: "Masuk", action: "/login", showPassword: showPassword, errMsg: errMsg,
		switchText: "Belum punya akun?", switchHref: "/register", switchLabel: "Daftar",
	})
}

// Register merender halaman pendaftaran (dev-only; route-nya tak ada di prod).
// showWorkspace: register mengumpulkan Nama Workspace (buat workspace baru).
func Register(errMsg string) g.Node {
	return authPage(authOpts{
		title: "Daftar", action: "/register", showPassword: true, showWorkspace: true, errMsg: errMsg,
		switchText: "Sudah punya akun?", switchHref: "/login", switchLabel: "Masuk",
	})
}

// authOpts = parameter authPage (struct agar tak jadi daftar argumen panjang).
type authOpts struct {
	title, action                       string
	showPassword, showWorkspace         bool
	errMsg                              string
	switchText, switchHref, switchLabel string
}

// authPage adalah kerangka bersama login & register: tombol Google + (opsional)
// form password (+ field Nama Workspace bila showWorkspace). Form = NATIVE POST
// (bukan Datastar @post): navigasi penuh via HTTP 303, bukan SSE — redirect SSE
// menyuntik <script> yang diblokir CSP proyek (script-src tanpa unsafe-inline).
func authPage(o authOpts) g.Node {
	card := []g.Node{googleButton()}
	if o.showPassword {
		if o.errMsg != "" {
			card = append(card, ui.Alert(ui.VariantDestructive, "auth-error", g.Text(o.errMsg)))
		}
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
	return h.Div(g.Group(body))
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

// passwordFields adalah form email/password NATIVE (dev-only). Submit = POST biasa
// ke action → server balas HTTP 303 (redirect). Input pakai name= (bukan Datastar
// bind); tombol type=submit. showWorkspace menambah field "Nama Workspace" di atas.
func passwordFields(action, submitLabel string, showWorkspace bool) g.Node {
	fields := []g.Node{}
	if showWorkspace {
		fields = append(fields, h.Div(
			h.Class("grid gap-2"),
			ui.Label("Nama Workspace", h.For("workspace")),
			ui.Input(
				h.ID("workspace"), h.Name("workspace"), h.Type("text"),
				h.Placeholder("mis. Acme Corp"),
			),
		))
	}
	fields = append(fields,
		h.Div(
			h.Class("grid gap-2"),
			ui.Label("Email", h.For("email")),
			ui.Input(
				h.ID("email"), h.Name("email"), h.Type("email"),
				h.Placeholder("nama@contoh.com"),
			),
		),
		h.Div(
			h.Class("grid gap-2"),
			ui.Label("Password", h.For("password")),
			ui.Input(
				h.ID("password"), h.Name("password"), h.Type("password"),
				h.Placeholder("••••••••"),
			),
		),
		h.Button(h.Type("submit"), h.Class("btn btn-primary w-full"), g.Text(submitLabel)),
	)
	// Form native: method POST + action → navigasi 303 (bukan SSE). w-full agar
	// tombol & field mengisi kartu (mobile-first).
	return h.FormEl(
		h.Method("post"), h.Action(action), h.Class("grid gap-4"),
		g.Group(fields),
	)
}
