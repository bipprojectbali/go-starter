package ui

import (
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Avatar merender foto profil dengan fallback inisial. Inisial SELALU dirender
// server-side (latar warna deterministik); <img> menimpanya bila berhasil load.
// Bila img 403/gagal (`onerror`), img dihapus → inisial muncul. Semua fallback
// LOKAL (tanpa gravatar/ui-avatars — anti leak PII + tahan network diblok).
//
// referrerpolicy="no-referrer" WAJIB: header global Referrer-Policy masih kirim
// origin → Google balas 403 intermiten; atribut per-img meng-override-nya.
func Avatar(avatarURL, name, email string, px int) g.Node {
	size := sizeClass(px)
	inner := []g.Node{
		h.Class("avatar-initials " + size),
		g.Attr("style", "background:"+deterministicColor(email)),
		g.Text(initials(name, email)),
	}
	nodes := []g.Node{
		h.Class("avatar-wrap " + size),
		h.Span(inner...),
	}
	if url := sizedAvatar(avatarURL, px); url != "" {
		nodes = append(nodes, h.Img(
			h.Class("avatar-img "+size),
			h.Src(url),
			h.Alt(""),
			g.Attr("referrerpolicy", "no-referrer"),
			g.Attr("loading", "lazy"),
			g.Attr("decoding", "async"),
			g.Attr("onerror", "this.remove()"),
		))
	}
	return h.Span(nodes...)
}

// initials mengambil 1–2 huruf dari nama (fallback email) untuk inisial avatar.
func initials(name, email string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		if i := strings.IndexByte(email, '@'); i > 0 {
			name = email[:i]
		} else {
			name = email
		}
	}
	fields := strings.Fields(strings.ReplaceAll(name, ".", " "))
	switch {
	case len(fields) == 0:
		return "?"
	case len(fields) == 1:
		return strings.ToUpper(firstRune(fields[0]))
	default:
		return strings.ToUpper(firstRune(fields[0]) + firstRune(fields[len(fields)-1]))
	}
}

func firstRune(s string) string {
	for _, r := range s {
		return string(r)
	}
	return ""
}

// deterministicColor memetakan email ke satu dari beberapa warna latar stabil
// (FNV hash sederhana) — avatar user sama selalu warna sama.
func deterministicColor(seed string) string {
	palette := []string{
		"#0ea5e9", "#8b5cf6", "#ec4899", "#f59e0b",
		"#10b981", "#ef4444", "#6366f1", "#14b8a6",
	}
	var hash uint32 = 2166136261
	for i := 0; i < len(seed); i++ {
		hash ^= uint32(seed[i])
		hash *= 16777619
	}
	return palette[hash%uint32(len(palette))]
}

// sizedAvatar menambahkan suffix ukuran Google (=sNN-c) ke URL base agar minta
// resolusi tepat. "" bila URL kosong.
func sizedAvatar(base string, px int) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	return base + "=s" + itoaPx(px*2) + "-c" // 2x untuk retina
}

// sizeClass memetakan px ke utility Tailwind (dipakai di app.css scan).
func sizeClass(px int) string {
	switch {
	case px <= 24:
		return "size-6"
	case px <= 32:
		return "size-8"
	case px <= 40:
		return "size-10"
	default:
		return "size-12"
	}
}

func itoaPx(n int) string {
	if n == 0 {
		return "0"
	}
	var b [10]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
