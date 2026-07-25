package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEmailList(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"a@x.com", 1},
		{"a@x.com,b@y.com", 2},
		{" a@x.com , b@y.com ", 2}, // trim
		{"a@x.com,,", 1},           // buang kosong
	}
	for _, c := range cases {
		if got := len(parseEmailList(c.in)); got != c.want {
			t.Errorf("parseEmailList(%q) = %d entri, want %d", c.in, got, c.want)
		}
	}
}

func TestIsSuperAdminEmail(t *testing.T) {
	c := &Config{SuperAdminEmails: parseEmailList("root@x.com, Owner@Y.com")}

	if !c.IsSuperAdminEmail("root@x.com") {
		t.Error("root@x.com harus super-admin")
	}
	// Case-insensitive: env & input di-lower.
	if !c.IsSuperAdminEmail("OWNER@y.com") {
		t.Error("perbandingan email harus case-insensitive")
	}
	if !c.IsSuperAdminEmail("  root@x.com  ") {
		t.Error("input harus di-trim")
	}
	if c.IsSuperAdminEmail("stranger@x.com") {
		t.Error("email non-env tidak boleh super-admin")
	}
}

func TestUnquote(t *testing.T) {
	cases := []struct{ in, want string }{
		{`"postgres://x"`, "postgres://x"}, // kutip ganda pembungkus dilepas
		{`'postgres://x'`, "postgres://x"}, // kutip tunggal pembungkus dilepas
		{"postgres://x", "postgres://x"},   // tanpa kutip → apa adanya
		{`"a"b"`, `a"b`},                   // hanya pasangan pembungkus terluar
		{`"`, `"`},                         // satu kutip → tak dianggap pasangan
		{"", ""},                           // kosong aman
		{`"mismatch'`, `"mismatch'`},       // kutip tak cocok → tak dilepas
	}
	for _, c := range cases {
		if got := unquote(c.in); got != c.want {
			t.Errorf("unquote(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

// TestLoadDotEnv_StripsQuotes: regresi bug "database bip does not exist" — nilai
// berkutip di .env (DATABASE_URL="postgres://...") harus dibaca TANPA kutip,
// jika tidak pgx gagal parse → database kosong.
func TestLoadDotEnv_StripsQuotes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "DATABASE_URL=\"postgres://u@localhost/db?sslmode=disable\"\n" +
		"PLAIN=value\n# komentar\nSINGLE='quoted'\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Pastikan bersih (LoadDotEnv tak menimpa env yang sudah ada).
	for _, k := range []string{"DATABASE_URL", "PLAIN", "SINGLE"} {
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}
	if err := LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if got := os.Getenv("DATABASE_URL"); got != "postgres://u@localhost/db?sslmode=disable" {
		t.Errorf("DATABASE_URL berkutip harus dilepas, got %q", got)
	}
	if got := os.Getenv("SINGLE"); got != "quoted" {
		t.Errorf("kutip tunggal harus dilepas, got %q", got)
	}
	if got := os.Getenv("PLAIN"); got != "value" {
		t.Errorf("nilai tanpa kutip apa adanya, got %q", got)
	}
}
