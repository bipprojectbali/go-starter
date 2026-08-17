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
	// Brand = nama aplikasi di header. DIOPER dari handler, tak pernah ditulis
	// di sini: view yang menyimpan nama aplikasi berarti setiap project turunan
	// harus menyisir layer tampilan untuk menggantinya (konvensi view murni-data).
	Brand string
	// SEO = metadata mesin-pencari & social-share (lihat seo.go). Diisi handler
	// (butuh base URL + path). Title-nya diselaraskan dengan <title> di Layout.
	SEO SEO
}

// Layout membungkus konten halaman dengan HTML5 lengkap: head, CSS, nav, dan
// runtime Datastar (vendored). Dipakai untuk full-page render (§4.4 jalur 1).
func Layout(d LayoutData, body ...g.Node) g.Node {
	cssPath := d.CSSPath
	if cssPath == "" {
		cssPath = "/static/app.css"
	}
	// Selaraskan judul kartu-share dengan <title> dokumen: bila handler tak
	// menyetel SEO.Title sendiri, pakai Title halaman — supaya og:title,
	// twitter:title, dan <title> tak pernah berbeda.
	seo := d.SEO
	if seo.Title == "" {
		seo.Title = d.Title
	}
	return c.HTML5(c.HTML5Props{
		Title:    d.Title,
		Language: "id",
		Head:     headNodes(cssPath, seo),
		Body: []g.Node{
			// Latar = base-200; permukaan (card) = base-100 → kartu menonjol.
			// Token relatif daisyUI: hierarki ini benar otomatis di semua tema.
			h.Class("min-h-screen bg-base-200 text-base-content"),
			nav(d.UserEmail, d.AvatarURL, d.Brand),
			h.Main(h.Class("mx-auto max-w-2xl p-6"), g.Group(body)),
			// Modal konfirmasi logout — hanya relevan bila ada nav (user login).
			logoutModal(d.UserEmail),
		},
	})
}

// headNodes = isi <head> bersama (SEO, favicon, CSS, Datastar). Dipakai Layout &
// AppShell agar tak duplikat (DRY). SEO dioper terpisah dari cssPath karena
// AppShell (halaman privat) selalu noindex, sedang Layout melayani halaman publik.
func headNodes(cssPath string, seo SEO) []g.Node {
	nodes := []g.Node{
		// Warna bar browser mobile mengikuti latar dasar aplikasi (base-200 gelap).
		metaName("theme-color", "#0a0a0a"),
		// Web App Manifest → installable (PWA-lite) + ikon di layar utama.
		h.Link(h.Rel("manifest"), h.Href("/manifest.webmanifest")),
		h.Link(h.Rel("icon"), h.Type("image/svg+xml"), h.Href("/static/favicon.svg")),
		// apple-touch-icon: iOS mengabaikan SVG favicon; SVG tetap dicoba, fallback
		// PNG tak ada di template (tambahkan bila perlu tampilan iOS sempurna).
		h.Link(g.Attr("rel", "apple-touch-icon"), h.Href("/static/favicon.svg")),
	}
	// Metadata SEO (robots, description, Open Graph, Twitter Card).
	nodes = append(nodes, seoNodes(seo)...)
	nodes = append(nodes,
		// app.css sudah berisi daisyUI (plugin Tailwind) — satu file, satu preflight.
		h.Link(h.Rel("stylesheet"), h.Href(cssPath)),
		// theme.js SINKRON (bukan defer): set data-theme sebelum paint → no-FOUC.
		h.Script(h.Src("/static/theme.js")),
		// dropdown.js: close-on-outside-click untuk <details.dropdown> (menu akun,
		// Tema, switcher). Tak butuh pre-paint → defer.
		h.Script(h.Src("/static/dropdown.js"), h.Defer()),
		h.Script(h.Src("/static/datastar.js"), h.Type("module"), h.Defer()),
	)
	return nodes
}

// nav menampilkan bar atas. Tombol logout + avatar muncul hanya bila user login.
func nav(userEmail, avatarURL, brand string) g.Node {
	if userEmail == "" {
		// Belum login (mis. halaman login) — tanpa nav, tapi tetap sediakan
		// pemilih tema mengambang di pojok agar tema bisa diganti sebelum login.
		return h.Div(h.Class("fixed top-4 right-4 z-20"), ThemeToggle())
	}
	return h.Header(
		h.Class("border-b border-base-300"),
		data.Signals(map[string]any{"logoutConfirm": false}),
		h.Div(
			h.Class("mx-auto max-w-2xl p-4 flex items-center justify-between"),
			h.A(h.Href("/"), h.Class("font-semibold"), g.Text(brand)),
			h.Div(
				h.Class("flex items-center gap-3"),
				ThemeToggle(),
				// Menu akun (avatar+email dipicu → Ganti akun / Keluar). Komponen
				// & rasional sama dengan footer sidebar (userMenu di shelluser.go).
				userMenu(avatarURL, userEmail, false),
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
		"Anda akan keluar dari sesi ini.", "Keluar", "/logout")
}
