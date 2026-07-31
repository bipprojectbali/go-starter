package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// pagecursor_test.go — cursor keyset yang lewat query string.
//
// Yang dijaga di sini adalah hal-hal yang gagalnya SENYAP: cursor rusak yang
// menghasilkan halaman kosong alih-alih halaman pertama, dan presisi waktu yang
// hilang saat dibulatkan sehingga satu baris terlewat di sambungan halaman.

func ts(t *testing.T, s string) pgtype.Timestamptz {
	t.Helper()
	tm, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("parse waktu: %v", err)
	}
	return pgtype.Timestamptz{Time: tm, Valid: true}
}

func reqAfter(after string) *http.Request {
	url := "/dev/users"
	if after != "" {
		url += "?after=" + after
	}
	return httptest.NewRequest(http.MethodGet, url, nil)
}

// TestPageCursor_BolakBalik: nilai yang dirakit harus terbaca kembali utuh.
// Presisi mikrodetik Postgres wajib selamat — dibulatkan ke detik, dua baris
// yang dibuat dalam detik yang sama akan saling menutupi di sambungan halaman.
func TestPageCursor_BolakBalik(t *testing.T) {
	at := ts(t, "2026-07-30T10:11:12.123456Z")
	raw := formatCursor(at, 42)

	gotAt, gotID := pageCursor(reqAfter(raw))
	if gotID != 42 {
		t.Errorf("id = %d, want 42", gotID)
	}
	if !gotAt.Time.Equal(at.Time) {
		t.Errorf("waktu = %v, want %v — presisi hilang di sambungan halaman", gotAt.Time, at.Time)
	}
}

// TestPageCursor_RusakJadiHalamanPertama: cursor datang dari URL yang bisa
// disunting, dipendekkan pengirim pesan, atau tersimpan sebagai bookmark lama.
// Yang rusak harus jatuh ke halaman pertama — BUKAN ke halaman kosong, yang
// terbaca sebagai "tak ada user sama sekali" dan mengundang laporan bug.
func TestPageCursor_RusakJadiHalamanPertama(t *testing.T) {
	firstAt, firstID := firstPageCursor()

	for _, raw := range []string{
		"",              // tak ada parameter
		"bukanangka_1",  // bagian waktu bukan angka
		"123_bukanid",   // bagian id bukan angka
		"1234567890",    // tanpa pemisah
		"_",             // pemisah saja
		"999_999_extra", // bagian berlebih
	} {
		gotAt, gotID := pageCursor(reqAfter(raw))
		if gotID != firstID || gotAt.InfinityModifier != firstAt.InfinityModifier {
			t.Errorf("after=%q harus jatuh ke halaman pertama, got (%v, %d)", raw, gotAt, gotID)
		}
	}
}

// TestSplitPage_MenandaiAdaLagi: query mengambil pageSize+1; baris lebih itu
// HANYA penanda dan tak boleh ikut dirender.
func TestSplitPage_MenandaiAdaLagi(t *testing.T) {
	type row struct {
		at pgtype.Timestamptz
		id int64
	}
	key := func(r row) (pgtype.Timestamptz, int64) { return r.at, r.id }

	rows := make([]row, 0, pageSize+1)
	for i := range pageSize + 1 {
		rows = append(rows, row{at: ts(t, "2026-07-30T10:00:00Z"), id: int64(i + 1)})
	}

	got, next := splitPage(rows, key)
	if len(got) != pageSize {
		t.Errorf("baris tampil = %d, want %d — baris penanda tak boleh ikut dirender", len(got), pageSize)
	}
	if next == "" {
		t.Fatal("masih ada baris berikutnya, cursor tak boleh kosong")
	}
	// Cursor menunjuk baris TERAKHIR yang ditampilkan, bukan baris penanda:
	// halaman berikutnya dimulai tepat sesudah yang sudah dilihat user.
	_, gotID, ok := parseCursor(next)
	if !ok {
		t.Fatalf("cursor tak terbaca: %q", next)
	}
	if gotID != int64(pageSize) {
		t.Errorf("cursor menunjuk id %d, want %d (baris terakhir yang TAMPIL)", gotID, pageSize)
	}
}

// TestSplitPage_HalamanTerakhirTanpaCursor: tanpa ini tombol "Berikutnya" tetap
// muncul di ujung daftar lalu berujung halaman kosong.
func TestSplitPage_HalamanTerakhirTanpaCursor(t *testing.T) {
	type row struct{ id int64 }
	key := func(r row) (pgtype.Timestamptz, int64) {
		return pgtype.Timestamptz{Valid: true}, r.id
	}
	for _, n := range []int{0, 1, pageSize} {
		rows := make([]row, n)
		got, next := splitPage(rows, key)
		if len(got) != n {
			t.Errorf("n=%d: baris tampil = %d, want %d", n, len(got), n)
		}
		if next != "" {
			t.Errorf("n=%d: halaman terakhir tak boleh punya cursor, got %q", n, next)
		}
	}
}
