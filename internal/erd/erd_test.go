package erd

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMermaid_Output menguji rendering teks Mermaid dari Schema in-memory
// (pure, tanpa DB).
func TestMermaid_Output(t *testing.T) {
	s := Schema{
		Tables: []Table{
			{Name: "users", Columns: []Column{
				{Name: "id", Type: "bigint", PK: true},
				{Name: "email", Type: "text"},
			}},
			{Name: "todos", Columns: []Column{
				{Name: "id", Type: "bigint", PK: true},
				{Name: "user_id", Type: "bigint", FK: true},
			}},
		},
		Relations: []Relation{{From: "todos", To: "users", Col: "user_id"}},
	}
	out := s.Mermaid()

	if !strings.HasPrefix(out, "erDiagram") {
		t.Errorf("harus mulai dgn erDiagram:\n%s", out)
	}
	// Relasi child→parent.
	if !strings.Contains(out, "users }o--|| todos : user_id") {
		t.Errorf("relasi FK harus ada:\n%s", out)
	}
	// PK & FK ditandai.
	if !strings.Contains(out, "bigint id PK") {
		t.Errorf("PK harus ditandai:\n%s", out)
	}
	if !strings.Contains(out, "bigint user_id FK") {
		t.Errorf("FK harus ditandai:\n%s", out)
	}
}

func TestMermaidType(t *testing.T) {
	cases := map[string]string{
		"text":                     "text",
		"character varying":        "character_varying",
		"timestamp with time zone": "timestamp_with_time_zone",
		"":                         "unknown",
	}
	for in, want := range cases {
		if got := mermaidType(in); got != want {
			t.Errorf("mermaidType(%q)=%q, want %q", in, got, want)
		}
	}
}

// TestIntrospect_LiveDB memverifikasi introspeksi terhadap DB nyata (skip bila
// TEST_DATABASE_URL kosong). Mengharap tabel inti + FK ke users terdeteksi.
func TestIntrospect_LiveDB(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tak di-set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	s, err := Introspect(ctx, pool)
	if err != nil {
		t.Fatalf("Introspect: %v", err)
	}
	// Tabel users harus ada + punya PK id.
	var users *Table
	for i := range s.Tables {
		if s.Tables[i].Name == "users" {
			users = &s.Tables[i]
		}
		if s.Tables[i].Name == "goose_db_version" {
			t.Error("tabel bookkeeping goose tak boleh masuk")
		}
	}
	if users == nil {
		t.Fatal("tabel users harus terdeteksi")
	}
	hasPK := false
	for _, c := range users.Columns {
		if c.Name == "id" && c.PK {
			hasPK = true
		}
	}
	if !hasPK {
		t.Error("users.id harus terdeteksi sbg PK")
	}
	// Ada relasi FK (mis. todos/oauth_accounts → users).
	if len(s.Relations) == 0 {
		t.Error("harus ada relasi FK terdeteksi")
	}
	// Mermaid render tak kosong.
	if !strings.HasPrefix(s.Mermaid(), "erDiagram") {
		t.Error("Mermaid output invalid")
	}
}
