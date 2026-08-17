package handler

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// wellknownHandler menyiapkan Handler minimal (tanpa DB) — endpoint SEO tingkat
// situs tak menyentuh database.
func wellknownHandler() *Handler {
	return &Handler{Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestRobots_TxtDisallowPrivateAndSitemap(t *testing.T) {
	prev := appBaseURL
	SetAppBaseURL("https://acme.test")
	t.Cleanup(func() { appBaseURL = prev })

	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	w := httptest.NewRecorder()
	wellknownHandler().Robots(w, req)

	body := w.Body.String()
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("robots content-type = %q", ct)
	}
	for _, want := range []string{
		"User-agent: *",
		"Disallow: /dev",
		"Disallow: /w/",
		"Sitemap: https://acme.test/sitemap.xml",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("robots.txt kurang %q:\n%s", want, body)
		}
	}
}

func TestSitemap_XMLPublicURLsOnly(t *testing.T) {
	prev := appBaseURL
	SetAppBaseURL("https://acme.test")
	t.Cleanup(func() { appBaseURL = prev })

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	w := httptest.NewRecorder()
	wellknownHandler().Sitemap(w, req)

	body := w.Body.String()
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/xml") {
		t.Errorf("sitemap content-type = %q", ct)
	}
	for _, want := range []string{
		`<?xml version="1.0"`,
		"<urlset",
		"<loc>https://acme.test/</loc>",
		"<loc>https://acme.test/login</loc>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap kurang %q:\n%s", want, body)
		}
	}
	// Area privat TAK boleh muncul di sitemap.
	for _, unwanted := range []string{"/dev", "/w/", "/notifications"} {
		if strings.Contains(body, "<loc>https://acme.test"+unwanted) {
			t.Errorf("sitemap tak boleh memuat area privat %q:\n%s", unwanted, body)
		}
	}
}

func TestManifest_UsesAppName(t *testing.T) {
	prevName := appName
	SetAppName("Acme Corp Platform")
	t.Cleanup(func() { appName = prevName })

	req := httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil)
	w := httptest.NewRecorder()
	wellknownHandler().Manifest(w, req)

	body := w.Body.String()
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/manifest+json") {
		t.Errorf("manifest content-type = %q", ct)
	}
	for _, want := range []string{
		`"name": "Acme Corp Platform"`,
		`"short_name": "Acme Corp Pl"`, // dipotong ~12 char
		`"display": "standalone"`,
		`"start_url": "/"`,
		`/static/favicon.svg`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("manifest kurang %q:\n%s", want, body)
		}
	}
}

// TestSetAppDescription_KosongTakMenimpa: default deskripsi tak boleh hilang bila
// APP_DESCRIPTION kosong (snippet mesin pencari selalu ada).
func TestSetAppDescription_KosongTakMenimpa(t *testing.T) {
	prev := appDescription
	t.Cleanup(func() { appDescription = prev })

	SetAppDescription("   ") // whitespace = kosong
	if appDescription != prev {
		t.Errorf("deskripsi kosong tak boleh menimpa default, got %q", appDescription)
	}
	SetAppDescription("Deskripsi baru")
	if appDescription != "Deskripsi baru" {
		t.Errorf("deskripsi valid harus di-set, got %q", appDescription)
	}
}

// TestPublicSEO_AbsoluteURLs: publicSEO merakit canonical & og:image absolut dari
// base URL request.
func TestPublicSEO_AbsoluteURLs(t *testing.T) {
	prev := appBaseURL
	SetAppBaseURL("https://acme.test")
	t.Cleanup(func() { appBaseURL = prev })

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	seo := publicSEO(req, "Masuk", "", "/login")
	if seo.CanonicalURL != "https://acme.test/login" {
		t.Errorf("canonical = %q", seo.CanonicalURL)
	}
	if seo.ImageURL != "https://acme.test/static/og-image.png" {
		t.Errorf("og:image = %q", seo.ImageURL)
	}
	if seo.NoIndex {
		t.Error("halaman publik tak boleh noindex")
	}
	if seo.Description == "" {
		t.Error("deskripsi kosong harus jatuh ke default")
	}
}
