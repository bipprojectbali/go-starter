package ui

// seo.go — metadata mesin-pencari & social-share (Open Graph, Twitter Card).
//
// View MURNI-DATA (konvensi layout.go): file ini TIDAK tahu nama aplikasi,
// alamat publik, maupun deskripsi — semuanya DIOPER lewat SEO. Handler yang
// merakit nilai absolut (base URL + path + config), sebab hanya di sana request
// dan config tersedia; menyimpannya di sini berarti tiap project turunan harus
// menyisir layer tampilan untuk menggantinya.
//
// Kenapa terpusat di satu fungsi (seoNodes): meta OG/Twitter itu SALING
// bergantung — og:title, twitter:title, dan <title> harus konsisten atau kartu
// share menampilkan judul berbeda dari halamannya. Satu sumber = mustahil
// berbeda.

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// SEO memuat metadata satu halaman untuk mesin pencari & pratinjau social-share.
// Semua field opsional: yang kosong tak menghasilkan tag (tag kosong = bising
// yang bisa membingungkan crawler). Nilai default diisi handler, bukan di sini.
type SEO struct {
	// Title = judul halaman (dipakai <title>, og:title, twitter:title). Bila
	// kosong, seoNodes memakai apa adanya — pemanggil (Layout) menyetelnya dari
	// Title dokumen agar ketiganya konsisten.
	Title string

	// Description = ringkasan halaman (meta description, og:description,
	// twitter:description). ~150–160 karakter ideal untuk snippet Google.
	Description string

	// CanonicalURL = URL absolut kanonik halaman ini (mis.
	// "https://app.example.com/login"). Mencegah duplikat konten (query string,
	// trailing slash, host www vs non-www dianggap satu). Juga jadi og:url.
	CanonicalURL string

	// ImageURL = URL ABSOLUT gambar pratinjau (og:image, twitter:image). Wajib
	// absolut — crawler Facebook/Twitter tak mengikuti path relatif. Rasio 1.91:1
	// (1200×630) = standar kartu besar.
	ImageURL string

	// SiteName = nama situs (og:site_name) — muncul di atas judul pada kartu share.
	SiteName string

	// Type = og:type. "website" untuk halaman umum, "article" untuk konten.
	// Kosong → "website".
	Type string

	// Locale = og:locale (mis. "id_ID"). Kosong → "id_ID" (bahasa aplikasi = id).
	Locale string

	// NoIndex = true → robots "noindex, nofollow": halaman TAK boleh masuk indeks
	// mesin pencari. Dipakai untuk halaman PRIVAT (dashboard, /dev, /w/{slug}) —
	// membocorkannya ke Google percuma (butuh login) dan berisiko mengekspos
	// struktur internal. Publik (landing, login) = false → boleh diindeks.
	NoIndex bool

	// TwitterSite = handle Twitter/X situs (mis. "@acme") untuk atribusi kartu.
	// Kosong → tag dilewati.
	TwitterSite string
}

// seoNodes merender seluruh tag <meta>/<link> SEO untuk <head>. Aman dipanggil
// dengan SEO kosong: hanya tag yang punya nilai yang keluar.
func seoNodes(s SEO) []g.Node {
	ogType := s.Type
	if ogType == "" {
		ogType = "website"
	}
	locale := s.Locale
	if locale == "" {
		locale = "id_ID"
	}

	nodes := []g.Node{
		// robots: kontrol indeksasi. Halaman privat = noindex,nofollow.
		metaName("robots", robotsValue(s.NoIndex)),
	}

	if s.Description != "" {
		// c.HTML5 sudah menulis <meta name="description"> bila Props.Description
		// diisi; kita SENGAJA tak mengoper lewat sana (Layout memakai Head kustom)
		// agar semua metadata SEO hidup di satu tempat, bukan tersebar.
		nodes = append(nodes, metaName("description", s.Description))
	}
	if s.CanonicalURL != "" {
		nodes = append(nodes, h.Link(h.Rel("canonical"), h.Href(s.CanonicalURL)))
	}

	// Open Graph (Facebook, LinkedIn, WhatsApp, Slack, dsb.).
	nodes = append(nodes,
		metaProperty("og:type", ogType),
		metaProperty("og:locale", locale),
	)
	if s.SiteName != "" {
		nodes = append(nodes, metaProperty("og:site_name", s.SiteName))
	}
	if s.Title != "" {
		nodes = append(nodes, metaProperty("og:title", s.Title))
	}
	if s.Description != "" {
		nodes = append(nodes, metaProperty("og:description", s.Description))
	}
	if s.CanonicalURL != "" {
		nodes = append(nodes, metaProperty("og:url", s.CanonicalURL))
	}
	if s.ImageURL != "" {
		nodes = append(nodes,
			metaProperty("og:image", s.ImageURL),
			// Dimensi eksplisit → crawler tak perlu mengunduh gambar untuk tahu
			// ukurannya (pratinjau muncul lebih cepat, tak "melompat").
			metaProperty("og:image:width", "1200"),
			metaProperty("og:image:height", "630"),
			metaProperty("og:image:alt", s.Title),
		)
	}

	// Twitter/X Card. summary_large_image = kartu gambar besar (butuh og:image).
	card := "summary"
	if s.ImageURL != "" {
		card = "summary_large_image"
	}
	nodes = append(nodes, metaName("twitter:card", card))
	if s.TwitterSite != "" {
		nodes = append(nodes, metaName("twitter:site", s.TwitterSite))
	}
	if s.Title != "" {
		nodes = append(nodes, metaName("twitter:title", s.Title))
	}
	if s.Description != "" {
		nodes = append(nodes, metaName("twitter:description", s.Description))
	}
	if s.ImageURL != "" {
		nodes = append(nodes,
			metaName("twitter:image", s.ImageURL),
			metaName("twitter:image:alt", s.Title),
		)
	}

	return nodes
}

// robotsValue memetakan flag privat → nilai direktif robots.
func robotsValue(noIndex bool) string {
	if noIndex {
		return "noindex, nofollow"
	}
	// index,follow + petunjuk snippet penuh (default sehat untuk halaman publik).
	return "index, follow, max-image-preview:large"
}

// metaName = <meta name=... content=...> (description, robots, twitter:*).
func metaName(name, content string) g.Node {
	return h.Meta(h.Name(name), h.Content(content))
}

// metaProperty = <meta property=... content=...> (Open Graph). gomponents tak
// punya helper Property(), jadi atribut dirakit manual — sama sahnya.
func metaProperty(property, content string) g.Node {
	return h.Meta(g.Attr("property", property), h.Content(content))
}
