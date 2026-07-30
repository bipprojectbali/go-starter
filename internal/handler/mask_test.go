package handler

import (
	"strings"
	"testing"
)

// mask_test.go — penyamaran email. Yang dijaga bukan bentuk kosmetiknya,
// melainkan dua sifat yang membuat penyamaran ini berguna sekaligus aman:
// alamatnya tak bisa dipakai menghubungi orangnya, tapi barisnya masih bisa
// DIBEDAKAN dari baris lain (tabel users tak punya kolom nama — email adalah
// satu-satunya penanda).

func TestMaskEmail(t *testing.T) {
	cases := []struct {
		in   string
		want string
		why  string
	}{
		{"malikkurosaki@gmail.com", "mal•••@gmail.com",
			"bagian lokal panjang: 3 huruf pertama cukup untuk membedakan orang"},
		{"bip.production.js@gmail.com", "bip•••@gmail.com",
			"titik di bagian lokal tak boleh lolos ikut tampil"},
		{"abcd@x.io", "ab•••@x.io",
			"lokal 4 huruf: memperlihatkan 3 dari 4 praktis tak menyamarkan apa pun"},
		{"ab@x.io", "a•••@x.io",
			"lokal 2 huruf: sisakan satu"},
		{"a@x.io", "a•••@x.io",
			"lokal 1 huruf: tak bisa lebih pendek lagi"},
		{"bukan-email", "•••",
			"struktur tak dikenali → sembunyikan penuh, jangan pura-pura menyamarkan"},
		{"", "•••", "kosong"},
		{"@x.io", "•••", "bagian lokal kosong"},
	}
	for _, c := range cases {
		if got := maskEmail(c.in); got != c.want {
			t.Errorf("maskEmail(%q) = %q, want %q — %s", c.in, got, c.want, c.why)
		}
	}
}

// TestMaskEmail_TakMembocorkanPanjang: panjang bagian lokal mempersempit ruang
// tebak, jadi jumlah titik harus TETAP — bukan sejumlah karakter yang disembunyikan.
func TestMaskEmail_TakMembocorkanPanjang(t *testing.T) {
	pendek := maskEmail("abcd@x.io")
	panjang := maskEmail("abcdefghijklmnopqrstuvwxyz@x.io")

	if strings.Count(pendek, "•") != strings.Count(panjang, "•") {
		t.Errorf("jumlah titik harus sama (%q vs %q) — panjang alamat tak boleh terbaca",
			pendek, panjang)
	}
}

// TestMaskEmail_DomainDipertahankan: domain adalah yang membedakan rekan satu
// organisasi dari orang luar — justru pertanyaan itu yang membuat daftar anggota
// berguna bagi anggota biasa. Menyamarkannya membuang manfaatnya tanpa menambah
// perlindungan yang berarti.
func TestMaskEmail_DomainDipertahankan(t *testing.T) {
	got := maskEmail("seseorang@perusahaan.co.id")
	if !strings.HasSuffix(got, "@perusahaan.co.id") {
		t.Errorf("domain harus utuh, got %q", got)
	}
}

// TestMaskEmail_TakBisaDipakaiMenghubungi: inti perlindungannya. Hasil samaran
// TIDAK BOLEH sama dengan alamat aslinya — kalau sama, penyamarannya cuma hiasan.
func TestMaskEmail_TakBisaDipakaiMenghubungi(t *testing.T) {
	for _, in := range []string{
		"malikkurosaki@gmail.com",
		"bip.production.js@gmail.com",
		"ab@x.io",
		"a@x.io",
	} {
		if got := maskEmail(in); got == in {
			t.Errorf("maskEmail(%q) mengembalikan alamat ASLI — tak menyamarkan apa pun", in)
		}
	}
}
