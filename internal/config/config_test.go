package config

import "testing"

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
