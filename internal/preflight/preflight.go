// Package preflight memeriksa lingkungan sebelum aplikasi melayani request,
// dan — yang lebih penting — MENJELASKAN apa yang harus dilakukan bila ada yang
// kurang.
//
// Alasannya bukan kenyamanan. Pesan `database "x" does not exist (SQLSTATE
// 3D000)` sudah benar secara harfiah, tapi ia menyembunyikan hal yang justru
// dibutuhkan penerimanya: nama apa yang dicari, database mana yang SEBENARNYA
// ada di server itu, dan perintah persis untuk memperbaikinya. Ketiganya sudah
// diketahui program — ia cuma tak mengatakannya.
//
// Prinsipnya mengikuti § "Konfigurasi produksi: gagal keras" di CLAUDE.md: yang
// berbahaya bila salah tetap menggagalkan boot; yang bisa diturunkan otomatis
// tak diminta ke manusia. Yang ditambahkan di sini cuma satu: kalau memang
// harus gagal, gagallah dengan petunjuk.
package preflight

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// dialTimeout membatasi lama pemeriksaan yang menyentuh jaringan. Pendek dengan
// sengaja: preflight berjalan di jalur boot, dan menunggu 30 detik untuk
// mengetahui Postgres mati membuat orang menekan Ctrl-C sebelum pesannya
// sempat muncul.
const dialTimeout = 3 * time.Second

// ErrDatabaseMissing menandai database yang belum dibuat — dibedakan dari
// kegagalan koneksi lain karena INI yang bisa diperbaiki otomatis di dev.
var ErrDatabaseMissing = errors.New("database belum dibuat")

// DBTarget = bagian DSN yang dibutuhkan untuk memeriksa & membuat database.
type DBTarget struct {
	DSN    string
	DBName string
	Host   string
	Port   string
}

// ParseDSN mengurai DSN Postgres jadi bagian yang kita butuhkan.
func ParseDSN(dsn string) (DBTarget, error) {
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return DBTarget{}, fmt.Errorf("DATABASE_URL tak bisa diurai: %w", err)
	}
	return DBTarget{
		DSN:    dsn,
		DBName: cfg.Database,
		Host:   cfg.Host,
		Port:   fmt.Sprint(cfg.Port),
	}, nil
}

// CheckDatabase memastikan database di DSN bisa dihubungi.
//
// Mengembalikan ErrDatabaseMissing (terbungkus) khusus untuk kasus "database
// belum dibuat", supaya pemanggil bisa memutuskan sendiri: dev membuatnya,
// production menolak.
func CheckDatabase(ctx context.Context, t DBTarget) error {
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, t.DSN)
	if err == nil {
		_ = conn.Close(ctx)
		return nil
	}
	// SQLSTATE 3D000 = invalid_catalog_name, yaitu "databasenya tak ada". Dicek
	// lewat kode, bukan teks pesan: teks berubah antar-versi & antar-locale
	// Postgres, sementara kodenya bagian dari standar.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "3D000" {
		return fmt.Errorf("%w: %q", ErrDatabaseMissing, t.DBName)
	}
	return err
}

// CreateDatabase membuat database yang belum ada, dengan menyambung ke
// database `postgres` di server yang sama.
//
// HANYA dipanggil di dev (lihat main.go). Di production ini justru berbahaya:
// DSN yang salah ketik akan lahir sebagai database kosong yang tampak sehat,
// dan aplikasi mulai melayani dengan data yang "hilang" — kegagalan senyap
// yang persis dihindari § "gagal keras" di CLAUDE.md.
func CreateDatabase(ctx context.Context, t DBTarget) error {
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	admin, err := pgx.Connect(ctx, adminDSN(t.DSN))
	if err != nil {
		return fmt.Errorf("sambung ke database 'postgres' untuk membuat %q: %w", t.DBName, err)
	}
	defer admin.Close(ctx)

	// pgx.Identifier menangani kutip & escape — nama database boleh memuat
	// tanda hubung (mis. "vanguard-matrix"), yang tanpa kutip adalah sintaks
	// tak sah dan gagal dengan pesan yang menyalahkan tempat lain.
	if _, err := admin.Exec(ctx,
		"CREATE DATABASE "+pgx.Identifier{t.DBName}.Sanitize()); err != nil {
		return fmt.Errorf("buat database %q: %w", t.DBName, err)
	}
	return nil
}

// adminDSN mengganti nama database di DSN dengan `postgres` (database yang
// selalu ada), mempertahankan host/port/kredensial/parameter lainnya.
func adminDSN(dsn string) string {
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return dsn
	}
	// Dirakit ulang dari komponen, bukan disunting sebagai string: DSN bisa
	// berbentuk URL maupun key=value, dan pencarian-ganti teks akan mengenai
	// nama yang kebetulan sama di bagian lain (mis. password).
	var b strings.Builder
	b.WriteString("postgres://")
	if cfg.User != "" {
		b.WriteString(cfg.User)
		if cfg.Password != "" {
			b.WriteByte(':')
			b.WriteString(cfg.Password)
		}
		b.WriteByte('@')
	}
	b.WriteString(net.JoinHostPort(cfg.Host, fmt.Sprint(cfg.Port)))
	b.WriteString("/postgres")
	if cfg.TLSConfig == nil {
		b.WriteString("?sslmode=disable")
	}
	return b.String()
}

// SimilarDatabases mencari database yang NAMANYA MIRIP dengan yang dicari.
//
// Ini inti nilai paket ini. Kekeliruan yang sebenarnya hampir selalu berupa
// beda tipis — tanda hubung vs garis bawah, akhiran `_test` yang tertinggal,
// nama lama sebelum project di-rename. Menampilkan yang mirip mengubah "kenapa
// tak ada?" jadi "oh, saya salah ketik" tanpa penerimanya perlu membuka psql.
//
// Best-effort: gagal mencari TIDAK boleh menggagalkan apa pun, sebab ini
// pelengkap pesan — bukan pemeriksaan.
func SimilarDatabases(ctx context.Context, t DBTarget) []string {
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, adminDSN(t.DSN))
	if err != nil {
		return nil
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx,
		`SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil && similar(t.DBName, name) {
			out = append(out, name)
		}
	}
	return out
}

// similar melaporkan apakah dua nama database "mirip" untuk keperluan saran.
//
// Bukan jarak Levenshtein: yang dicari bukan kemiripan umum melainkan pola
// kekeliruan yang BENAR-BENAR terjadi di sini — pemisah kata yang berbeda
// (`-` vs `_`), dan awalan/akhiran yang tertinggal (`_test`, nama lama). Aturan
// sederhana yang menjelaskan dirinya sendiri lebih berguna daripada ambang
// angka yang harus ditebak-tebak.
func similar(want, got string) bool {
	if want == got {
		return false // yang sama persis bukan "mirip" — ia yang dicari
	}
	norm := func(s string) string {
		return strings.ToLower(strings.NewReplacer("-", "", "_", "", ".", "").Replace(s))
	}
	a, b := norm(want), norm(got)
	return a == b || strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}
