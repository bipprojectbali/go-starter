package handler

import (
	"fmt"
	"net/http"
	"strings"
)

// wellknown.go — endpoint SEO tingkat-situs: robots.txt, sitemap.xml, dan Web
// App Manifest. Semuanya PUBLIK (tanpa auth) dan dirakit dari base URL + config,
// bukan file statik — agar host di canonical/sitemap selalu benar di belakang
// proxy dan ikut nama aplikasi (APP_NAME) tanpa disunting tangan.

// Robots menyajikan /robots.txt. Mengizinkan crawl halaman publik, MELARANG area
// privat (di balik login — percuma di-crawl, dan menyembunyikan struktur
// internal), lalu menunjuk sitemap absolut agar mesin pencari menemukannya.
func (h *Handler) Robots(w http.ResponseWriter, r *http.Request) {
	base := baseFrom(r)
	// Disallow area privat/aksi. Bukan kontrol keamanan (tetap dijaga auth),
	// hanya menjaga crawler tak buang anggaran crawl ke halaman yang pasti 302
	// ke /login atau butuh token.
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	for _, p := range []string{"/dev", "/w/", "/notifications", "/invite/", "/workspace/", "/account/", "/api/", "/logout"} {
		fmt.Fprintf(&b, "Disallow: %s\n", p)
	}
	b.WriteString("Allow: /$\n")
	fmt.Fprintf(&b, "\nSitemap: %s\n", absURL(base, "/sitemap.xml"))

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

// Sitemap menyajikan /sitemap.xml — daftar URL PUBLIK yang layak diindeks.
// Hanya halaman tanpa-login: landing, masuk, daftar. Halaman privat sengaja
// tak masuk (noindex + Disallow robots). URL absolut dari base URL.
func (h *Handler) Sitemap(w http.ResponseWriter, r *http.Request) {
	base := baseFrom(r)
	// Path publik + prioritas relatif. Register hanya relevan di mode multi &
	// dev, tapi tetap didaftar publik (crawler abaikan yang 404/redirect).
	pages := []struct {
		path     string
		priority string
	}{
		{"/", "1.0"},
		{"/login", "0.5"},
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, p := range pages {
		fmt.Fprintf(&b, "  <url>\n    <loc>%s</loc>\n    <priority>%s</priority>\n  </url>\n",
			absURL(base, p.path), p.priority)
	}
	b.WriteString("</urlset>\n")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

// Manifest menyajikan /manifest.webmanifest (Web App Manifest) → aplikasi bisa
// "Add to Home Screen" (PWA-lite) dengan nama & ikon yang benar. name diambil
// dari APP_NAME (via appName) agar tiap deployment memasang identitasnya sendiri.
func (h *Handler) Manifest(w http.ResponseWriter, r *http.Request) {
	// short_name dibatasi ~12 char (rekomendasi) agar tak terpotong di layar utama.
	short := appName
	if len(short) > 12 {
		short = short[:12]
	}
	// JSON dirakit manual (kecil, tetap; menghindari alokasi encoder untuk 6 field).
	// Ikon SVG maskable — satu file melayani semua ukuran.
	body := fmt.Sprintf(`{
  "name": %q,
  "short_name": %q,
  "description": %q,
  "start_url": "/",
  "display": "standalone",
  "background_color": "#0a0a0a",
  "theme_color": "#0a0a0a",
  "icons": [
    { "src": "/static/favicon.svg", "type": "image/svg+xml", "sizes": "any", "purpose": "any maskable" }
  ]
}`, appName, short, appDescription)

	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	_, _ = w.Write([]byte(body))
}
