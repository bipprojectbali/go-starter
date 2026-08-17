package ui

import (
	"strings"
	"testing"

	g "maragu.dev/gomponents"
)

func renderSEO(s SEO) string {
	var sb strings.Builder
	for _, n := range seoNodes(s) {
		_ = n.Render(&sb)
	}
	return sb.String()
}

// TestSEO_PublicPage: halaman publik menghasilkan set metadata lengkap —
// description, canonical, Open Graph, Twitter Card — dan robots index,follow.
func TestSEO_PublicPage(t *testing.T) {
	out := renderSEO(SEO{
		Title:        "Acme",
		Description:  "Ringkasan produk Acme.",
		CanonicalURL: "https://acme.test/",
		ImageURL:     "https://acme.test/static/og-image.png",
		SiteName:     "Acme",
		TwitterSite:  "@acme",
	})
	for _, want := range []string{
		`name="robots" content="index, follow, max-image-preview:large"`,
		`name="description" content="Ringkasan produk Acme."`,
		`rel="canonical" href="https://acme.test/"`,
		`property="og:type" content="website"`,
		`property="og:locale" content="id_ID"`,
		`property="og:site_name" content="Acme"`,
		`property="og:title" content="Acme"`,
		`property="og:url" content="https://acme.test/"`,
		`property="og:image" content="https://acme.test/static/og-image.png"`,
		`property="og:image:width" content="1200"`,
		`name="twitter:card" content="summary_large_image"`,
		`name="twitter:site" content="@acme"`,
		`name="twitter:title" content="Acme"`,
		`name="twitter:image"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("SEO publik kurang %q:\n%s", want, out)
		}
	}
}

// TestSEO_NoIndex: halaman privat = robots noindex,nofollow (isinya di balik
// login — mengindeksnya percuma & membocorkan struktur). Tanpa gambar → kartu
// twitter turun ke "summary".
func TestSEO_NoIndex(t *testing.T) {
	out := renderSEO(SEO{Title: "Dashboard", NoIndex: true})
	if !strings.Contains(out, `name="robots" content="noindex, nofollow"`) {
		t.Errorf("privat harus noindex,nofollow:\n%s", out)
	}
	if !strings.Contains(out, `name="twitter:card" content="summary"`) {
		t.Errorf("tanpa gambar → twitter card summary:\n%s", out)
	}
	// Tanpa nilai → tak ada tag description/canonical/og:image bocor.
	for _, unwanted := range []string{`name="description"`, `rel="canonical"`, `property="og:image"`} {
		if strings.Contains(out, unwanted) {
			t.Errorf("SEO kosong tak boleh emit %q:\n%s", unwanted, out)
		}
	}
}

// TestLayout_SEOTitleFallback: bila handler tak menyetel SEO.Title, Layout
// menyelaraskannya dengan <title> dokumen → og:title == <title>.
func TestLayout_SEOTitleFallback(t *testing.T) {
	var sb strings.Builder
	Layout(LayoutData{Title: "Halaman X"}, g.Text("body")).Render(&sb)
	out := sb.String()
	if !strings.Contains(out, "<title>Halaman X</title>") {
		t.Errorf("title dokumen hilang:\n%s", out)
	}
	if !strings.Contains(out, `property="og:title" content="Halaman X"`) {
		t.Errorf("og:title harus selaras dgn <title>:\n%s", out)
	}
	// Head bersama juga membawa theme-color & manifest.
	for _, want := range []string{`name="theme-color"`, `rel="manifest" href="/manifest.webmanifest"`} {
		if !strings.Contains(out, want) {
			t.Errorf("head kurang %q:\n%s", want, out)
		}
	}
}

// TestAppShell_AlwaysNoIndex: halaman dashboard SELALU noindex (privat).
func TestAppShell_AlwaysNoIndex(t *testing.T) {
	out := renderShell("/dev/users")
	if !strings.Contains(out, `name="robots" content="noindex, nofollow"`) {
		t.Errorf("AppShell harus noindex:\n%s", out)
	}
}
