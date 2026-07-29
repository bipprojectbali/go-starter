package db

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// rlscheck_test.go — pemeriksaan berbasis BUKTI menggantikan janji "env sudah
// diisi benar". Yang dijaga di sini: setiap cara RLS bisa lolos harus terdeteksi,
// dan pesannya harus menyebut sebabnya (menebak di area ini berakhir pada
// kebocoran data).

// TestRLSStatus_Binds memetakan tiap kombinasi ke keputusan. Tabel, bukan DB:
// kombinasi seperti "superuser sekaligus pemilik tabel" mahal disiapkan di
// Postgres nyata, sementara logikanya justru yang perlu dikunci.
func TestRLSStatus_Binds(t *testing.T) {
	cases := []struct {
		nama  string
		st    RLSStatus
		binds bool
	}{
		{
			"role terbatas — inilah yang diinginkan",
			RLSStatus{User: "app_rw", Forced: true},
			true,
		},
		{
			"superuser: SELALU bypass, bahkan dengan FORCE RLS",
			RLSStatus{User: "postgres", Superuser: true, Forced: true},
			false,
		},
		{
			"atribut BYPASSRLS eksplisit",
			RLSStatus{User: "svc", BypassRLS: true, Forced: true},
			false,
		},
		{
			"pemilik tabel TANPA force: bypass policy diam-diam",
			RLSStatus{User: "owner", OwnsTables: true, Forced: false},
			false,
		},
		{
			"pemilik tabel DENGAN force: tertahan (itu guna FORCE)",
			RLSStatus{User: "owner", OwnsTables: true, Forced: true},
			true,
		},
		{
			// Kasus yang paling mudah terlewat: FORCE RLS dimatikan di migrasi
			// mendatang. Role-nya benar, tapi perlindungannya hilang.
			"non-owner tanpa force: policy tetap berlaku baginya",
			RLSStatus{User: "app_rw", OwnsTables: false, Forced: false},
			true,
		},
	}
	for _, c := range cases {
		if got := c.st.Binds(); got != c.binds {
			t.Errorf("%s: Binds()=%v, want %v", c.nama, got, c.binds)
		}
		// Yang tidak mengikat WAJIB bisa menjelaskan diri: pesan kosong memaksa
		// operator menebak, dan menebak di sini berakhir pada kebocoran.
		if !c.binds && c.st.Reason() == "" {
			t.Errorf("%s: tidak mengikat tapi Reason() kosong", c.nama)
		}
		if c.binds && c.st.Reason() != "" {
			t.Errorf("%s: mengikat tapi Reason() terisi (%q)", c.nama, c.st.Reason())
		}
	}
}

// TestReason_MenyebutRole: pesan harus menyebut role yang BENAR-BENAR dipakai —
// itulah satu-satunya petunjuk yang membedakan "salah DSN" dari "salah privilege".
func TestReason_MenyebutRole(t *testing.T) {
	st := RLSStatus{User: "bip", Superuser: true}
	if !strings.Contains(st.Reason(), "bip") {
		t.Errorf("Reason harus menyebut role yang dipakai, got %q", st.Reason())
	}
}

// TestCheckRLS_TerhadapDatabaseNyata: query-nya harus benar-benar jalan di
// Postgres, bukan cuma masuk akal di atas kertas. Koneksi test = superuser, jadi
// jawaban yang diharapkan adalah "TIDAK mengikat" — dan itu justru membuktikan
// pemeriksaannya jujur.
func TestCheckRLS_TerhadapDatabaseNyata(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tidak di-set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	st, err := CheckRLS(ctx, pool, "audit_logs")
	if err != nil {
		t.Fatalf("CheckRLS: %v", err)
	}
	if st.User == "" {
		t.Error("current_user harus terbaca")
	}
	if !st.Forced {
		t.Error("FORCE RLS harus aktif di audit_logs (migrasi 00007) — " +
			"tanpanya pemilik tabel bypass policy diam-diam")
	}
	// Koneksi test memakai superuser → wajib terdeteksi TIDAK mengikat.
	if st.Superuser && st.Binds() {
		t.Error("superuser TIDAK BOLEH dilaporkan sebagai terikat RLS")
	}
}

// TestCheckRLS_TabelTakAda: salah nama tabel harus jadi error yang terlihat,
// bukan diam-diam dianggap aman.
func TestCheckRLS_TabelTakAda(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tidak di-set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	if _, err := CheckRLS(ctx, pool, "tabel_yang_tak_pernah_ada"); err == nil {
		t.Error("tabel probe tak ditemukan harus jadi error, bukan lolos senyap")
	}
}
