package dev

import (
	"strings"
	"testing"
)

func TestERDPage(t *testing.T) {
	src := "erDiagram\n    users }o--|| todos : user_id\n    users {\n        bigint id PK\n    }\n"
	var sb strings.Builder
	if err := ERDPage(src, 2).Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := sb.String()

	// Wadah mermaid + teks diagram (ter-escape aman).
	if !strings.Contains(out, `class="mermaid"`) {
		t.Errorf("harus ada <pre class=mermaid>:\n%s", out)
	}
	if !strings.Contains(out, "erDiagram") {
		t.Errorf("teks diagram harus ada:\n%s", out)
	}
	// Runtime + init dimuat.
	if !strings.Contains(out, "/static/mermaid.min.js") || !strings.Contains(out, "/static/erd.js") {
		t.Errorf("mermaid + init harus dimuat:\n%s", out)
	}
	// Jumlah tabel di deskripsi.
	if !strings.Contains(out, "2 tabel") {
		t.Errorf("jumlah tabel harus tampil:\n%s", out)
	}
}
