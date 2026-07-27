package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tablescroll_test.go — regresi mobile-first: <table> WAJIB dibungkus
// ui.TableScroll. Diukur di browser: satu tabel telanjang membuat halaman
// meluber 439px di viewport 375px (halaman ikut melar, bukan hanya tabel).
//
// Test STRUKTURAL (baca source), bukan render: bug ini muncul dari cara view
// DITULIS, dan tiap view punya signature berbeda sehingga menguji per-halaman
// akan cepat basi. Yang dijaga: aturan penulisan berlaku di SEMUA view.

// TestSemuaTabelDibungkusTableScroll memindai view gomponents dan menolak
// h.Table( yang tidak didahului ui.TableScroll( — pola yang bocor ke halaman.
func TestSemuaTabelDibungkusTableScroll(t *testing.T) {
	root := filepath.Join("..", "ui")
	var offenders []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(src), "\n") {
			if !strings.Contains(line, "h.Table(") {
				continue
			}
			// Pembungkus boleh di baris yang sama (ui.TableScroll(h.Table(...))
			// atau di baris tepat sebelumnya (dipisah gofmt).
			if strings.Contains(line, "TableScroll(") {
				continue
			}
			offenders = append(offenders, path+":"+itoa(i+1))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan view: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("<table> tanpa ui.TableScroll (akan mendorong lebar halaman di mobile):\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// TestTableScrollPunyaOverflowDanMinW menjaga KEDUA class tetap ada: overflow-x-auto
// saja tak cukup — tanpa min-w-0 pembungkus menolak menyusut (min-width:auto
// default) sehingga overflow jadi mubazir.
func TestTableScrollPunyaOverflowDanMinW(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("components.go"))
	if err != nil {
		t.Fatalf("baca components.go: %v", err)
	}
	for _, cls := range []string{"overflow-x-auto", "min-w-0"} {
		if !strings.Contains(string(src), cls) {
			t.Errorf("TableScroll wajib memakai %q", cls)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
