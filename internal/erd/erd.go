// Package erd membangun teks Mermaid erDiagram dari katalog Postgres (live DB).
// Dipakai panel dev /dev/erd untuk memvisualisasikan relasi tabel.
package erd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Column = satu kolom tabel dengan penanda PK/FK.
type Column struct {
	Name string
	Type string
	PK   bool
	FK   bool
}

// Table = satu tabel + kolomnya (terurut sesuai posisi ordinal).
type Table struct {
	Name    string
	Columns []Column
}

// Relation = relasi FK antar tabel (child → parent).
type Relation struct {
	From string // tabel anak (punya kolom FK)
	To   string // tabel induk (yang direferensi)
	Col  string // kolom FK di anak
}

// Schema = hasil introspeksi: tabel + relasi.
type Schema struct {
	Tables    []Table
	Relations []Relation
}

// Introspect membaca struktur tabel public dari information_schema + katalog
// pg untuk PK/FK. Mengabaikan tabel goose (goose_db_version) & casbin bila ada.
func Introspect(ctx context.Context, pool *pgxpool.Pool) (Schema, error) {
	var s Schema

	// Seluruh query di bawah menyaring `table_schema = current_schema()`, BUKAN
	// 'public' harfiah. Dengan nama harfiah, ERD tampil KOSONG di deployment mana
	// pun yang menaruh aplikasi di schema lain — halaman termuat rapi tanpa satu
	// tabel pun, yang terbaca seperti database kosong alih-alih filter yang
	// keliru. Terungkap saat template ini di-clone ke database baru.

	// 1. Tabel + kolom (urut ordinal), skip tabel bookkeeping.
	rows, err := pool.Query(ctx, `
		SELECT c.table_name, c.column_name, c.data_type
		FROM information_schema.columns c
		JOIN information_schema.tables t
		  ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		WHERE c.table_schema = current_schema()
		  AND t.table_type = 'BASE TABLE'
		  AND c.table_name NOT IN ('goose_db_version')
		ORDER BY c.table_name, c.ordinal_position`)
	if err != nil {
		return s, fmt.Errorf("erd: query columns: %w", err)
	}
	defer rows.Close()

	tableIdx := map[string]int{}
	for rows.Next() {
		var tbl, col, typ string
		if err := rows.Scan(&tbl, &col, &typ); err != nil {
			return s, err
		}
		i, ok := tableIdx[tbl]
		if !ok {
			i = len(s.Tables)
			tableIdx[tbl] = i
			s.Tables = append(s.Tables, Table{Name: tbl})
		}
		s.Tables[i].Columns = append(s.Tables[i].Columns, Column{Name: col, Type: typ})
	}
	if err := rows.Err(); err != nil {
		return s, err
	}

	// 2. Primary keys.
	if err := markConstraint(ctx, pool, "PRIMARY KEY", func(tbl, col string) {
		if i, ok := tableIdx[tbl]; ok {
			for j := range s.Tables[i].Columns {
				if s.Tables[i].Columns[j].Name == col {
					s.Tables[i].Columns[j].PK = true
				}
			}
		}
	}); err != nil {
		return s, err
	}

	// 3. Foreign keys → tandai kolom FK + kumpulkan relasi.
	fkRows, err := pool.Query(ctx, `
		SELECT tc.table_name, kcu.column_name, ccu.table_name AS foreign_table
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
		  ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = current_schema()`)
	if err != nil {
		return s, fmt.Errorf("erd: query fk: %w", err)
	}
	defer fkRows.Close()
	for fkRows.Next() {
		var tbl, col, ftbl string
		if err := fkRows.Scan(&tbl, &col, &ftbl); err != nil {
			return s, err
		}
		if i, ok := tableIdx[tbl]; ok {
			for j := range s.Tables[i].Columns {
				if s.Tables[i].Columns[j].Name == col {
					s.Tables[i].Columns[j].FK = true
				}
			}
		}
		s.Relations = append(s.Relations, Relation{From: tbl, To: ftbl, Col: col})
	}
	if err := fkRows.Err(); err != nil {
		return s, err
	}

	// Urutkan agar output deterministik (tabel & relasi).
	sort.Slice(s.Tables, func(i, j int) bool { return s.Tables[i].Name < s.Tables[j].Name })
	sort.Slice(s.Relations, func(i, j int) bool {
		if s.Relations[i].From != s.Relations[j].From {
			return s.Relations[i].From < s.Relations[j].From
		}
		return s.Relations[i].Col < s.Relations[j].Col
	})
	return s, nil
}

// markConstraint memanggil fn(table,column) untuk tiap kolom yang termasuk
// constraint bertipe tertentu (mis. PRIMARY KEY).
func markConstraint(ctx context.Context, pool *pgxpool.Pool, ctype string, fn func(tbl, col string)) error {
	rows, err := pool.Query(ctx, `
		SELECT tc.table_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = $1 AND tc.table_schema = current_schema()`, ctype)
	if err != nil {
		return fmt.Errorf("erd: query %s: %w", ctype, err)
	}
	defer rows.Close()
	for rows.Next() {
		var tbl, col string
		if err := rows.Scan(&tbl, &col); err != nil {
			return err
		}
		fn(tbl, col)
	}
	return rows.Err()
}

// Mermaid merender Schema jadi teks Mermaid erDiagram.
func (s Schema) Mermaid() string {
	var b strings.Builder
	b.WriteString("erDiagram\n")

	// Relasi: child }o--|| parent (banyak anak → satu induk).
	for _, r := range s.Relations {
		fmt.Fprintf(&b, "    %s }o--|| %s : %s\n", r.To, r.From, r.Col)
	}

	// Definisi tabel + kolom.
	for _, t := range s.Tables {
		fmt.Fprintf(&b, "    %s {\n", t.Name)
		for _, c := range t.Columns {
			key := ""
			switch {
			case c.PK:
				key = " PK"
			case c.FK:
				key = " FK"
			}
			// Mermaid: "type name key". Tipe dibersihkan (spasi → _) agar valid.
			fmt.Fprintf(&b, "        %s %s%s\n", mermaidType(c.Type), c.Name, key)
		}
		b.WriteString("    }\n")
	}
	return b.String()
}

// mermaidType menyederhanakan tipe Postgres agar aman jadi token Mermaid
// (tanpa spasi/tanda kurung yang memecah parser).
func mermaidType(t string) string {
	t = strings.ReplaceAll(t, " ", "_")
	t = strings.ReplaceAll(t, "(", "_")
	t = strings.ReplaceAll(t, ")", "")
	if t == "" {
		return "unknown"
	}
	return t
}
