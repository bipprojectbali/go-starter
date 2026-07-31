// Package testdb memberi tiap paket test schema Postgres-nya SENDIRI, sehingga
// `go test ./...` bisa berjalan paralel lagi.
//
// Masalah yang diselesaikan: seluruh test berbagi satu database, dan beberapa
// paket men-TRUNCATE tabel global saat setup. Paralel berarti satu paket
// menghapus data paket lain di tengah jalan — gagalnya tak konsisten, dan yang
// gagal bukan yang menyebabkan. Jalan keluar sementaranya `-p 1`, yang menukar
// masalah itu dengan test yang makin lambat seiring paket bertambah.
//
// Kenapa SCHEMA, bukan database per paket: schema hanya butuh satu CREATE
// (milidetik) dan ikut terbuang lewat DROP ... CASCADE, sementara database
// terpisah menuntut koneksi administratif, template, dan izin CREATEDB yang
// belum tentu dimiliki pemakai. Isolasinya cukup — search_path membuat tiap
// paket melihat tabelnya sendiri.
package testdb

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"go_starter/internal/database"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DSN mengembalikan alamat DB test, atau meng-skip test bila tak diset.
//
// Skip, bukan gagal: kontributor tanpa Postgres lokal tetap bisa menjalankan
// test yang murni logika, dan CI-lah yang menyetel variabelnya.
func DSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tidak di-set; lewati test yang butuh DB")
	}
	return dsn
}

// schemaName menurunkan nama schema dari nama paket pemanggil.
//
// Dari NAMA PAKET, bukan acak: schema yang tertinggal karena test dibunuh
// (Ctrl-C, timeout CI) harus bisa dikenali dan dibersihkan orang, bukan
// menumpuk sebagai "test_a3f9c1" yang tak berarti apa-apa. Nama yang sama juga
// berarti menjalankan ulang paket yang sama memakai schema itu lagi alih-alih
// menambah satu baru.
func schemaName(pkg string) string {
	clean := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 32
		default:
			return '_'
		}
	}, pkg)
	return "test_" + clean
}

// Pool menyiapkan schema terisolasi untuk paket ini, menjalankan seluruh migrasi
// di dalamnya, lalu mengembalikan pool yang search_path-nya sudah mengarah ke
// sana.
//
// pkg = nama paket pemanggil (mis. "handler"). Schema-nya dibuat ulang dari nol
// tiap kali TestMain berjalan, jadi test tak mewarisi sisa run sebelumnya —
// itulah yang dulu dicapai lewat TRUNCATE, kini tanpa menyentuh milik paket
// lain.
func Pool(ctx context.Context, pkg string) (*pgxpool.Pool, error) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		return nil, nil // pemanggil (TestMain) melewati setup; tiap test akan Skip
	}
	schema := schemaName(pkg)

	// Fase 1: bikin schema-nya, lewat koneksi biasa (search_path default).
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pool admin: %w", err)
	}
	defer admin.Close()
	// DROP dulu: sisa run sebelumnya (termasuk yang dibunuh di tengah jalan) tak
	// boleh mencemari yang ini. CASCADE karena isinya memang milik kita sendiri.
	if _, err := admin.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", pgx.Identifier{schema}.Sanitize())); err != nil {
		return nil, fmt.Errorf("drop schema %s: %w", schema, err)
	}
	if _, err := admin.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s", pgx.Identifier{schema}.Sanitize())); err != nil {
		return nil, fmt.Errorf("create schema %s: %w", schema, err)
	}

	// Fase 2: pool yang SELALU memakai schema itu.
	//
	// search_path lewat parameter koneksi, bukan `SET` manual di AfterConnect:
	// dengan begini ia ikut di-reset otomatis pada tiap koneksi baru yang dibuka
	// pool, termasuk yang menggantikan koneksi mati — sementara `SET` biasa bisa
	// hilang tanpa ada yang sadar.
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	// search_path HANYA schema ini — `public` sengaja TIDAK disertakan.
	//
	// Ini bukan soal kerapian melainkan syarat agar migrasinya jalan sama sekali.
	// goose mencari tabel versinya lewat search_path: dengan `public` di dalamnya
	// ia menemukan `goose_db_version` milik database utama, membacanya sebagai
	// "sudah versi terakhir", lalu tak menjalankan apa pun — schema baru tetap
	// KOSONG dan test gagal dengan "relation does not exist" yang menunjuk ke
	// mana-mana kecuali penyebabnya.
	//
	// Aman tanpa public: seluruh migrasi memakai tipe & fungsi bawaan
	// (pg_catalog selalu ada di search_path secara implisit), dan tak ada
	// ekstensi yang dipasang di sana.
	cfg.ConnConfig.RuntimeParams["search_path"] = schema

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pool: %w", err)
	}

	// Fase 3: migrasi DI DALAM schema itu. goose menaruh tabel versinya sendiri
	// di schema aktif juga, jadi tiap paket punya riwayat migrasinya sendiri dan
	// tak saling mengunci.
	if err := database.MigrateWithLock(ctx, pool, os.DirFS(repoRoot())); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrasi schema %s: %w", schema, err)
	}
	return pool, nil
}

// repoRoot mencari akar repo dari direktori kerja test (yang selalu berada di
// direktori paket-nya sendiri). Dicari lewat go.mod, bukan "../.." harfiah:
// jumlah tingkat berbeda antar paket, dan yang salah hitung gagal dengan pesan
// "no migrations found" yang tak menunjuk penyebabnya.
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for range 10 {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir
		}
		parent := dir + "/.."
		abs, err := os.Stat(parent)
		if err != nil || !abs.IsDir() {
			break
		}
		dir = parent
	}
	return "."
}

// Drop membuang schema paket ini. Dipanggil TestMain setelah semua test selesai.
//
// Best-effort: kegagalan membersihkan TAK boleh membuat test yang lulus jadi
// merah. Schema yang tertinggal akan di-DROP di awal run berikutnya.
func Drop(ctx context.Context, pkg string) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		return
	}
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return
	}
	defer admin.Close()
	_, _ = admin.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE",
		pgx.Identifier{schemaName(pkg)}.Sanitize()))
}
