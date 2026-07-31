package oauth

import (
	"strings"
	"testing"
)

// displayname_test.go — pembersihan claim `name` dari Google.
//
// Nilai ini USER-CONTROLLED: siapa pun bisa menyetel namanya di akun Google jadi
// apa saja, lalu nama itu tampil di daftar anggota SETIAP orang di workspace
// yang sama. Yang diuji di sini adalah batas-batas yang menahan penyalahgunaan
// tanpa menolak nama sungguhan.

func TestNormalizeDisplayName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want *string // nil = expect nil
		why  string
	}{
		{"kosong", "", nil, "tak ada nama = NULL, bukan string kosong yang merender baris hampa"},
		{"spasi saja", "   ", nil, "spasi bukan nama"},
		{"biasa", "Budi Santoso", ptr("Budi Santoso"), ""},
		{"trim tepi", "  Budi  ", ptr("Budi"), ""},
		{"runtun spasi diciutkan", "Budi     Santoso", ptr("Budi Santoso"),
			"spasi beruntun bisa dipakai menggeser kolom di tabel"},
		{"newline dibuang", "Budi\nAdmin", ptr("Budi Admin"),
			"nama satu baris yang menyelipkan newline merusak tata letak & bisa menyamar jadi baris terpisah"},
		{"tab & carriage return", "Budi\t\rSantoso", ptr("Budi Santoso"), ""},
		{"karakter kontrol dibuang", "Budi\x00\x07", ptr("Budi"), ""},
		{"non-Latin utuh", "李明", ptr("李明"), "nama non-Latin bukan kasus tepi"},
		{"emoji dipertahankan", "Budi 🎉", ptr("Budi 🎉"),
			"norak, tapi bukan ancaman — dan menolaknya berarti menebak nama siapa yang 'sah'"},
		{"hanya karakter kontrol", "\x00\x01", nil, "tak ada yang tersisa = NULL"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NormalizeDisplayName(c.in)
			switch {
			case c.want == nil && got != nil:
				t.Errorf("want nil, got %q — %s", *got, c.why)
			case c.want != nil && got == nil:
				t.Errorf("want %q, got nil — %s", *c.want, c.why)
			case c.want != nil && got != nil && *got != *c.want:
				t.Errorf("got %q, want %q — %s", *got, *c.want, c.why)
			}
		})
	}
}

// TestNormalizeDisplayName_PanjangDibatasi: tanpa batas, satu orang bisa
// mengirim nama sepanjang megabyte yang lalu kita simpan dan render di daftar
// anggota semua orang lain.
func TestNormalizeDisplayName_PanjangDibatasi(t *testing.T) {
	got := NormalizeDisplayName(strings.Repeat("a", maxDisplayName*3))
	if got == nil {
		t.Fatal("nama panjang harus dipotong, bukan dibuang")
	}
	if n := len([]rune(*got)); n > maxDisplayName {
		t.Errorf("panjang %d rune melebihi batas %d", n, maxDisplayName)
	}
}

// TestNormalizeDisplayName_PotongDiBatasRune: memotong pada byte ke-N membelah
// rune UTF-8 multi-byte di tengah, dan sisanya berakhir sebagai "?" di layar.
// Nama non-Latin yang panjang adalah tempat bug ini muncul, bukan kasus buatan.
func TestNormalizeDisplayName_PotongDiBatasRune(t *testing.T) {
	got := NormalizeDisplayName(strings.Repeat("あ", maxDisplayName+20))
	if got == nil {
		t.Fatal("want nama terpotong, got nil")
	}
	if !utf8Valid(*got) {
		t.Error("hasil potong bukan UTF-8 valid — rune terbelah di tengah")
	}
	if n := len([]rune(*got)); n != maxDisplayName {
		t.Errorf("dipotong jadi %d rune, want %d", n, maxDisplayName)
	}
}

func utf8Valid(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}
