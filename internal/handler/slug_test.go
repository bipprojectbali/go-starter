package handler

import (
	"testing"

	"go_starter/internal/db"
)

func TestSlugify(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Acme Corp", "acme-corp"},
		{"  Acme  Corp  ", "acme-corp"}, // spasi rangkap & tepi diringkas
		{"Acme, Inc.", "acme-inc"},      // simbol → pemisah, tak menumpuk '-'
		{"ACME", "acme"},                // lowercase
		{"Café 42", "caf-42"},           // non-ascii dibuang (jadi pemisah)
		{"", "workspace"},               // kosong → fallback
		{"!!!", "workspace"},            // simbol-saja → fallback
		{"--hi--", "hi"},                // trim '-' tepi
		{"a b c", "a-b-c"},              // spasi → '-'
	}
	for _, c := range cases {
		if got := slugify(c.in); got != c.want {
			t.Errorf("slugify(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

func TestEmailLocal(t *testing.T) {
	cases := []struct{ in, want string }{
		{"budi@acme.com", "budi"},
		{"budi", "budi"}, // tanpa '@'
		{"@x", "workspace"},
		{"", "workspace"},
	}
	for _, c := range cases {
		if got := emailLocal(c.in); got != c.want {
			t.Errorf("emailLocal(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

// TestUniqueSlug: slug pertama = base; tabrakan → base-2, base-3, ... (nama boleh
// duplikat, slug tidak). Butuh DB (TenantSlugExists). Seed via owner-pool (env.q).
func TestUniqueSlug(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()

	// setupTest sudah seed tenant slug "test". Buat "acme" → harus bebas.
	s1, err := uniqueSlug(ctx, env.q, "acme")
	if err != nil {
		t.Fatalf("uniqueSlug 1: %v", err)
	}
	if s1 != "acme" {
		t.Fatalf("slug pertama harus 'acme', got %q", s1)
	}
	// Materialkan "acme" lalu minta lagi → "acme-2".
	if _, err := env.q.CreateTenant(ctx, db.CreateTenantParams{Name: "Acme", Slug: "acme"}); err != nil {
		t.Fatalf("seed acme: %v", err)
	}
	s2, err := uniqueSlug(ctx, env.q, "acme")
	if err != nil {
		t.Fatalf("uniqueSlug 2: %v", err)
	}
	if s2 != "acme-2" {
		t.Fatalf("slug kedua harus 'acme-2', got %q", s2)
	}
	// Materialkan "acme-2" → berikutnya "acme-3".
	if _, err := env.q.CreateTenant(ctx, db.CreateTenantParams{Name: "Acme", Slug: "acme-2"}); err != nil {
		t.Fatalf("seed acme-2: %v", err)
	}
	s3, err := uniqueSlug(ctx, env.q, "acme")
	if err != nil {
		t.Fatalf("uniqueSlug 3: %v", err)
	}
	if s3 != "acme-3" {
		t.Fatalf("slug ketiga harus 'acme-3', got %q", s3)
	}
}
