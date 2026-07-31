package maintenance

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"go_starter/internal/settings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// tasks_test.go — pekerjaan pemeliharaan yang MENGHAPUS data permanen.
//
// Ini yang paling penting diuji di seluruh paket: kekeliruan di sini tak
// menghasilkan error, tak menghasilkan halaman rusak, dan tak bisa dibatalkan —
// ia hanya membuat data hilang lebih cepat daripada yang diputuskan siapa pun.

// testPool membuka pool ke DB test.
//
// SENGAJA TIDAK men-TRUNCATE. `go test ./...` menjalankan paket secara PARALEL
// di atas satu database, jadi membersihkan tabel global di sini menghapus data
// yang sedang dipakai paket lain — kegagalan yang muncul di test tak
// berhubungan, hanya saat dijalankan bersamaan, dan menuduh kode yang salah.
//
// Gantinya, tiap test menandai barisnya sendiri (lihat mark) dan hanya
// menghitung miliknya. Yang dipurge pun tak mengganggu paket lain: baris mereka
// selalu dibuat dengan created_at = sekarang, jauh di dalam batas retensi mana
// pun yang diuji di sini.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tidak di-set; lewati test yang butuh DB")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// mark = penanda unik per test, ditanam di metadata agar barisnya bisa dihitung
// terpisah dari milik test/paket lain yang berjalan bersamaan.
func mark(t *testing.T) string { return "uji:" + t.Name() }

// seedAudit menyisipkan satu jejak dengan created_at eksplisit, bertanda test
// pemanggilnya.
func seedAudit(t *testing.T, pool *pgxpool.Pool, action string, at time.Time) {
	t.Helper()
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO audit_logs (action, target_type, metadata, created_at)
		 VALUES ($1, 'platform', jsonb_build_object('mark', $2::text), $3)`,
		action, mark(t), at); err != nil {
		t.Fatalf("seed audit: %v", err)
	}
	// Sisa baris dibersihkan setelah test — yang dipurge memang sudah hilang,
	// yang selamat tak boleh menumpuk di DB bersama.
	t.Cleanup(func() {
		_, _ = pool.Exec(context.WithoutCancel(t.Context()),
			`DELETE FROM audit_logs WHERE metadata->>'mark' = $1`, mark(t))
	})
}

// countAudit menghitung HANYA baris milik test ini.
func countAudit(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var n int64
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM audit_logs WHERE metadata->>'mark' = $1`, mark(t)).Scan(&n); err != nil {
		t.Fatalf("hitung audit: %v", err)
	}
	return n
}

// countPurgedOwn menghitung berapa baris MILIK TEST INI yang hilang — dipakai
// alih-alih nilai balik purge, yang ikut menghitung baris paket lain bila
// kebetulan ada yang kedaluwarsa saat test berjalan.
func countPurgedOwn(t *testing.T, pool *pgxpool.Pool, sebelum int64) int64 {
	t.Helper()
	return sebelum - countAudit(t, pool)
}

// withRetention menyetel batas retensi untuk satu test lalu memulihkannya —
// settings adalah state PAKET, dan yang lupa dipulihkan meracuni test lain
// dengan gejala di tempat tak berhubungan.
func withRetention(t *testing.T, days string) {
	t.Helper()
	prev := settings.All()[settings.KeyAuditRetentionDays]
	settings.Set(settings.KeyAuditRetentionDays, days)
	t.Cleanup(func() { settings.Set(settings.KeyAuditRetentionDays, prev) })
}

// TestPurgeAudit_HanyaYangLewatBatas: sisi utama. Yang di dalam batas WAJIB
// selamat — purge yang terlalu rakus menghapus bukti yang masih dibutuhkan, dan
// tak ada cara mengetahuinya setelah terjadi.
func TestPurgeAudit_HanyaYangLewatBatas(t *testing.T) {
	pool := testPool(t)
	withRetention(t, "30")
	now := time.Now()

	seedAudit(t, pool, "workspace.delete", now.AddDate(0, 0, -100)) // jauh lewat batas
	seedAudit(t, pool, "workspace.delete", now.AddDate(0, 0, -31))  // lewat batas tipis
	seedAudit(t, pool, "auth.login", now.AddDate(0, 0, -29))        // MASIH di dalam
	seedAudit(t, pool, "auth.login", now)                           // baru

	if _, err := PurgeAuditLogs(pool)(t.Context()); err != nil {
		t.Fatalf("purge: %v", err)
	}
	// Dihitung dari baris MILIK test ini, bukan dari nilai balik purge: yang
	// terakhir ikut menghitung baris paket lain yang kebetulan kedaluwarsa.
	if hilang := countPurgedOwn(t, pool, 4); hilang != 2 {
		t.Errorf("harus menghapus 2 baris lewat batas, got %d", hilang)
	}
	if sisa := countAudit(t, pool); sisa != 2 {
		t.Errorf("jejak di dalam batas harus SELAMAT, sisa %d (want 2)", sisa)
	}
}

// TestPurgeAudit_AngkaDiLuarBatasDitolak: angka yang TERBACA tapi di luar batas
// (mis. seseorang menulis "1" langsung ke DB) tak boleh dipakai. Retensi 1 hari
// akan membuang hampir seluruh jejak dalam satu siklus, dan itu tak bisa
// dibatalkan — lebih baik tak membersihkan sampai orang memperbaiki nilainya.
func TestPurgeAudit_AngkaDiLuarBatasDitolak(t *testing.T) {
	pool := testPool(t)
	seedAudit(t, pool, "workspace.delete", time.Now().AddDate(-5, 0, 0))

	for _, buruk := range []string{"0", "-1", "1", "29", "99999"} {
		withRetention(t, buruk)
		n, err := PurgeAuditLogs(pool)(t.Context())
		if err == nil {
			t.Errorf("retensi %q harus ditolak dengan error, bukan dijalankan diam-diam", buruk)
		}
		if n != 0 {
			t.Errorf("retensi %q menghapus %d baris — tak boleh menyentuh apa pun", buruk, n)
		}
	}
	if countAudit(t, pool) != 1 {
		t.Error("jejak terhapus padahal batasnya tak masuk akal")
	}
}

// TestPurgeAudit_NilaiRusakJatuhKeDefault: nilai yang TAK BISA diparse sama
// sekali ("bukan-angka") diperlakukan seperti tak ada, sehingga batasnya jadi
// default 365 hari — dan pembersihan TETAP berjalan.
//
// Ini SENGAJA berbeda dari kasus di atas, dan bedanya penting: angka di luar
// batas adalah niat yang keliru (seseorang menulis "1"), sementara nilai rusak
// adalah ketiadaan nilai. Menghentikan pemeliharaan karena satu baris rusak
// berarti tabel tumbuh diam-diam sampai ada yang sadar berbulan-bulan kemudian —
// sedangkan jatuh ke 365 tak pernah menghapus LEBIH banyak daripada yang
// dimaksudkan siapa pun. Arah amannya jelas, jadi ia dipilih.
func TestPurgeAudit_NilaiRusakJatuhKeDefault(t *testing.T) {
	pool := testPool(t)
	withRetention(t, "bukan-angka")
	now := time.Now()

	seedAudit(t, pool, "workspace.delete", now.AddDate(-5, 0, 0)) // >365 hari
	seedAudit(t, pool, "auth.login", now.AddDate(0, 0, -100))     // <365 hari

	if _, err := PurgeAuditLogs(pool)(t.Context()); err != nil {
		t.Fatalf("nilai rusak harus jatuh ke default, bukan menghentikan pemeliharaan: %v", err)
	}
	if hilang := countPurgedOwn(t, pool, 2); hilang != 1 {
		t.Errorf("harus memakai default %d hari dan menghapus 1 baris, got %d",
			settings.DefaultAuditRetentionDays, hilang)
	}
	if countAudit(t, pool) != 1 {
		t.Error("jejak di dalam batas default harus selamat")
	}
}

// TestPurgeAudit_TakAdaYangLewatBatas: siklus yang tak menemukan apa-apa harus
// diam & sukses, bukan error — pemeliharaan yang mengeluh setiap hari melatih
// orang mengabaikan lognya.
func TestPurgeAudit_TakAdaYangLewatBatas(t *testing.T) {
	pool := testPool(t)
	withRetention(t, "365")
	seedAudit(t, pool, "auth.login", time.Now())

	if _, err := PurgeAuditLogs(pool)(t.Context()); err != nil {
		t.Errorf("siklus kosong harus sukses, err=%v", err)
	}
	if countAudit(t, pool) != 1 {
		t.Error("jejak baru tak boleh tersentuh")
	}
}

// --- purge tenant ---

// seedDeletedTenant membuat workspace yang sudah di-soft-delete pada waktu
// tertentu, dan mengembalikan id-nya.
func seedDeletedTenant(t *testing.T, pool *pgxpool.Pool, slug string, deletedAt time.Time) int64 {
	t.Helper()
	// Slug dibuat unik per test: DB test dipakai bersama paket lain yang jalan
	// paralel, dan slug punya unique index.
	uniq := slug + "-" + strings.ToLower(strings.ReplaceAll(t.Name(), "_", "-"))
	var id int64
	err := pool.QueryRow(t.Context(),
		`INSERT INTO tenants (name, slug, status, deleted_at)
		 VALUES ($1, $2, 'active', $3) RETURNING id`, slug, uniq, deletedAt).Scan(&id)
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.WithoutCancel(t.Context()), "DELETE FROM tenants WHERE id = $1", id)
	})
	return id
}

func tenantExists(t *testing.T, pool *pgxpool.Pool, id int64) bool {
	t.Helper()
	var n int64
	if err := pool.QueryRow(t.Context(),
		"SELECT count(*) FROM tenants WHERE id = $1", id).Scan(&n); err != nil {
		t.Fatalf("cek tenant: %v", err)
	}
	return n > 0
}

const grace = 30 * 24 * time.Hour

// TestPurgeTenant_HanyaYangHabisTenggang: workspace dalam masa tenggang WAJIB
// selamat — di situlah restore masih mungkin, dan menghapusnya lebih awal
// membatalkan janji yang jadi alasan soft-delete ada.
func TestPurgeTenant_HanyaYangHabisTenggang(t *testing.T) {
	pool := testPool(t)
	now := time.Now()

	habis := seedDeletedTenant(t, pool, "habis", now.AddDate(0, 0, -31))
	masih := seedDeletedTenant(t, pool, "masih", now.AddDate(0, 0, -29))

	if _, err := PurgeExpiredTenants(pool, grace, quietLog())(t.Context()); err != nil {
		t.Fatalf("purge tenant: %v", err)
	}
	if tenantExists(t, pool, habis) {
		t.Error("workspace yang habis tenggang harus terhapus permanen")
	}
	if !tenantExists(t, pool, masih) {
		t.Error("workspace yang MASIH dalam tenggang terhapus — restore jadi mustahil")
	}
}

// TestPurgeTenant_TakMenyentuhYangHidup: yang tak pernah dihapus sama sekali
// (deleted_at NULL) tak boleh ikut. Ini yang paling mahal bila salah.
func TestPurgeTenant_TakMenyentuhYangHidup(t *testing.T) {
	pool := testPool(t)
	var hidup int64
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO tenants (name, slug, status) VALUES ('Hidup','hidup','active') RETURNING id`,
	).Scan(&hidup); err != nil {
		t.Fatalf("seed tenant hidup: %v", err)
	}
	seedDeletedTenant(t, pool, "matisudah", time.Now().AddDate(0, 0, -100))

	if _, err := PurgeExpiredTenants(pool, grace, quietLog())(t.Context()); err != nil {
		t.Fatalf("purge tenant: %v", err)
	}
	if !tenantExists(t, pool, hidup) {
		t.Fatal("workspace AKTIF terhapus oleh pemeliharaan — kerusakan terparah yang mungkin")
	}
}

// TestPurgeTenant_JejakAuditSelamat: audit_logs.tenant_id ON DELETE SET NULL
// (0005 §6) — bukti tak boleh lenyap bersama yang dibuktikan. Purge otomatis
// menjadikan jaminan itu benar-benar diuji: sebelum ada pemicu, ia hanya janji.
func TestPurgeTenant_JejakAuditSelamat(t *testing.T) {
	pool := testPool(t)
	id := seedDeletedTenant(t, pool, "berjejak", time.Now().AddDate(0, 0, -60))
	// created_at = SEKARANG: yang diuji di sini adalah CASCADE dari purge tenant,
	// bukan retensi. Jejak bertanggal lama akan ikut terbawa purge audit dan
	// membuat test ini gagal karena sebab yang sama sekali berbeda.
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO audit_logs (action, target_type, target_id, tenant_id, metadata)
		 VALUES ('workspace.delete', 'workspace', $1, $1, jsonb_build_object('mark', $2::text))`,
		id, mark(t)); err != nil {
		t.Fatalf("seed audit: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.WithoutCancel(t.Context()),
			`DELETE FROM audit_logs WHERE metadata->>'mark' = $1`, mark(t))
	})

	if _, err := PurgeExpiredTenants(pool, grace, quietLog())(t.Context()); err != nil {
		t.Fatalf("purge tenant: %v", err)
	}
	if tenantExists(t, pool, id) {
		t.Fatal("workspace harus terpurge")
	}
	if countAudit(t, pool) != 1 {
		t.Fatal("jejak audit ikut terhapus — bukti tak boleh lenyap bersama yang dibuktikan")
	}
	// tenant_id-nya jadi NULL, tapi barisnya tetap terbaca.
	var tenantID *int64
	if err := pool.QueryRow(t.Context(),
		`SELECT tenant_id FROM audit_logs WHERE metadata->>'mark' = $1`, mark(t)).Scan(&tenantID); err != nil {
		t.Fatalf("baca audit: %v", err)
	}
	if tenantID != nil {
		t.Errorf("tenant_id harus NULL setelah workspace-nya dipurge, got %d", *tenantID)
	}
}

// TestPurgeTenant_TakAdaKandidat: siklus kosong = sukses diam.
func TestPurgeTenant_TakAdaKandidat(t *testing.T) {
	pool := testPool(t)
	if _, err := PurgeExpiredTenants(pool, grace, quietLog())(t.Context()); err != nil {
		t.Errorf("siklus kosong harus sukses, err=%v", err)
	}
}

// TestTryLock_HanyaSatuInstance: advisory lock mencegah dua instance mengerjakan
// purge yang sama bersamaan. Yang KALAH harus LEWAT, bukan antre lalu mengulang
// pekerjaan yang baru saja selesai.
func TestTryLock_HanyaSatuInstance(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()

	release, ok := tryLock(ctx, pool)
	if !ok {
		t.Fatal("instance pertama harus dapat lock")
	}
	if _, ok2 := tryLock(ctx, pool); ok2 {
		t.Error("instance kedua TAK boleh dapat lock saat yang pertama memegangnya")
	}
	release()

	// Setelah dilepas, giliran berikutnya bisa masuk — kalau tidak, satu siklus
	// yang gagal akan mengunci pemeliharaan selamanya.
	release2, ok3 := tryLock(ctx, pool)
	if !ok3 {
		t.Fatal("lock tak pernah dilepas — pemeliharaan akan macet permanen")
	}
	release2()
}

// TestPurgeAudit_TerkunciTakMengerjakanApaPun: instance yang kalah lomba
// mengembalikan 0 tanpa error, BUKAN menghapus ulang.
func TestPurgeAudit_TerkunciTakMengerjakanApaPun(t *testing.T) {
	pool := testPool(t)
	withRetention(t, "30")
	seedAudit(t, pool, "workspace.delete", time.Now().AddDate(0, 0, -100))

	release, ok := tryLock(t.Context(), pool)
	if !ok {
		t.Fatal("gagal mengambil lock untuk simulasi")
	}
	defer release()

	n, err := PurgeAuditLogs(pool)(t.Context())
	if err != nil || n != 0 {
		t.Errorf("instance yang kalah lomba harus lewat diam-diam, got n=%d err=%v", n, err)
	}
	if countAudit(t, pool) != 1 {
		t.Error("instance terkunci tetap menghapus — lock-nya tak berfungsi")
	}
}
