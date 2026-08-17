package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/ui"
)

// seo.go — perakitan metadata SEO (base URL + path + config) untuk halaman
// PUBLIK. Terpusat di sini, bukan di view: hanya handler yang punya request
// (untuk canonical URL) dan config (nama & deskripsi aplikasi). View tetap
// murni-data (ui.SEO), file ini mengisinya.

// appDescription = deskripsi default aplikasi (meta description & og:description
// halaman publik tanpa deskripsi khusus). Di-inject dari config (APP_DESCRIPTION)
// via SetAppDescription — pola setter global yang sama dengan SetAppName.
//
// Default menyebut nilai jual template; project turunan menggantinya lewat env
// tanpa menyisir kode (Rule 15 — jangan hardcode identitas project).
var appDescription = "Starter web full-stack Go: cepat, ringan, single binary. " +
	"Datastar, gomponents, dan PostgreSQL — modern, agent-friendly."

// SetAppDescription menetapkan deskripsi aplikasi (dipanggil dari main saat
// startup). Kosong dibiarkan default agar snippet tak pernah hilang sama sekali.
func SetAppDescription(d string) {
	if strings.TrimSpace(d) != "" {
		appDescription = d
	}
}

// ogImagePath = path (relatif ke base URL) gambar pratinjau social-share
// default. Aset statik ter-embed; di-set absolut saat merakit SEO. Bila file tak
// ada, kartu share cukup tampil tanpa gambar (tag di-skip di view bila kosong).
const ogImagePath = "/static/og-image.png"

// publicSEO merakit metadata SEO untuk halaman PUBLIK (landing, login, undangan).
// canonicalPath = path kanonik halaman (mis. "/login"); URL absolut dirakit dari
// base URL (baseFrom) agar canonical & og:url benar di belakang proxy.
//
// desc kosong → pakai deskripsi aplikasi default. Halaman publik = boleh
// diindeks (NoIndex tetap false).
func publicSEO(r *http.Request, title, desc, canonicalPath string) ui.SEO {
	base := baseFrom(r)
	description := desc
	if strings.TrimSpace(description) == "" {
		description = appDescription
	}
	return ui.SEO{
		Title:        title,
		Description:  description,
		CanonicalURL: absURL(base, canonicalPath),
		ImageURL:     absURL(base, ogImagePath),
		SiteName:     appName,
		Type:         "website",
		Locale:       "id_ID",
		NoIndex:      false,
	}
}

// absURL menyambung base URL (tanpa trailing slash) dengan path (diawali "/").
// Path kosong → base saja (halaman root "/").
func absURL(base, path string) string {
	if base == "" {
		return path
	}
	if path == "" || path == "/" {
		return base + "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}
