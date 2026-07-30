package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// rlscheck.go — MEMBUKTIKAN isolasi tenant mengikat, alih-alih memercayai janji
// bahwa env sudah diisi benar.
//
// Sebelumnya aplikasi hanya menuntut APP_DATABASE_URL terisi di production. Itu
// menangkap "lupa isi", tapi TIDAK menangkap kesalahan yang jauh lebih mungkin:
// mengisinya dengan DSN yang sama seperti DATABASE_URL ("biar aman, samakan
// saja"). Terukur di database nyata: query yang lupa `WHERE tenant_id` dengan
// GUC ter-set ke satu tenant tetap mengembalikan 82 baris dari 15 tenant —
// sementara orangnya merasa sudah mengamankan.
//
// Yang bisa ditanyakan ke database jawabannya pasti, dan tak bisa dibohongi.

// RLSStatus = hasil pemeriksaan koneksi runtime.
type RLSStatus struct {
	User       string // role yang benar-benar dipakai koneksi
	Superuser  bool   // superuser SELALU bypass RLS, bahkan dengan FORCE
	BypassRLS  bool   // atribut BYPASSRLS eksplisit
	OwnsTables bool   // pemilik tabel bypass policy — kecuali FORCE RLS aktif
	Forced     bool   // FORCE RLS aktif di tabel ber-tenant
}

// Binds melaporkan apakah RLS benar-benar MENGIKAT koneksi ini.
//
// Superuser & BYPASSRLS lolos tanpa syarat — tak ada yang bisa menahannya.
// Pemilik tabel juga bypass policy, TAPI hanya bila FORCE RLS tidak aktif; kode
// ini mengaktifkannya di migrasi 00007, jadi kasus itu tertutup selama flag-nya
// masih menyala (yang ikut diperiksa di sini, karena migrasi bisa saja diubah).
func (s RLSStatus) Binds() bool {
	if s.Superuser || s.BypassRLS {
		return false
	}
	if s.OwnsTables && !s.Forced {
		return false
	}
	return true
}

// Reason menjelaskan KENAPA RLS tak mengikat, dalam kalimat yang bisa langsung
// ditindaklanjuti. Pesan galat yang hanya bilang "konfigurasi salah" memaksa
// orang menebak — dan menebak di area ini berakhir pada kebocoran data.
func (s RLSStatus) Reason() string {
	switch {
	case s.Superuser:
		return fmt.Sprintf("terhubung sebagai %q yang SUPERUSER — superuser selalu bypass RLS", s.User)
	case s.BypassRLS:
		return fmt.Sprintf("role %q punya atribut BYPASSRLS", s.User)
	case s.OwnsTables && !s.Forced:
		return fmt.Sprintf("role %q memiliki tabel ber-tenant dan FORCE RLS tidak aktif", s.User)
	default:
		return ""
	}
}

// rlsProbeSQL menanyakan keadaan yang menentukan apakah RLS mengikat. Semuanya
// dari katalog Postgres — tak ada yang bisa dibohongi konfigurasi.
const rlsProbeSQL = `
	SELECT current_user,
	       r.rolsuper,
	       r.rolbypassrls,
	       c.relowner = r.oid AS owns_table,
	       c.relforcerowsecurity
	FROM pg_roles r, pg_class c
	WHERE r.rolname = current_user AND c.relname = $1
`

// CheckRLSTx memeriksa pengikatan RLS DI DALAM transaksi yang sudah berjalan —
// yaitu setelah SET LOCAL ROLE menurunkan haknya ke app_rw.
//
// Inilah yang benar untuk ditanyakan sejak jadi satu DSN: pool telanjang memang
// masih owner (migrasi membutuhkannya), jadi memeriksa di sana akan selalu
// menjawab "tidak mengikat" padahal setiap query aplikasi terikat. Yang diukur
// harus keadaan yang sama dengan yang dijalani query sungguhan.
//
// probeTable = tabel ber-tenant mana pun; semuanya dikonfigurasi seragam.
func CheckRLSTx(ctx context.Context, q *Queries, probeTable string) (RLSStatus, error) {
	var s RLSStatus
	err := q.db.QueryRow(ctx, rlsProbeSQL, probeTable).
		Scan(&s.User, &s.Superuser, &s.BypassRLS, &s.OwnsTables, &s.Forced)
	if err != nil {
		return s, fmt.Errorf("periksa pengikatan RLS: %w", err)
	}
	return s, nil
}

// CheckRLS memeriksa pool secara langsung, TANPA penurunan hak. Dipakai test
// untuk membuktikan pemeriksaannya jujur: koneksi owner harus dilaporkan TIDAK
// mengikat, sebab kalau tidak, seluruh mekanisme ini hanya stempel.
func CheckRLS(ctx context.Context, pool *pgxpool.Pool, probeTable string) (RLSStatus, error) {
	var s RLSStatus
	err := pool.QueryRow(ctx, `
		SELECT current_user,
		       r.rolsuper,
		       r.rolbypassrls,
		       c.relowner = r.oid AS owns_table,
		       c.relforcerowsecurity
		FROM pg_roles r, pg_class c
		WHERE r.rolname = current_user AND c.relname = $1
	`, probeTable).Scan(&s.User, &s.Superuser, &s.BypassRLS, &s.OwnsTables, &s.Forced)
	if err != nil {
		return s, fmt.Errorf("periksa pengikatan RLS: %w", err)
	}
	return s, nil
}
