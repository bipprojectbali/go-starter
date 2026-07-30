package db

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// privileges_test.go — keputusan 0007: SATU DSN, hak diturunkan per-transaksi.
//
// Ini menggantikan dua koneksi (owner + app_rw) yang dulu dibutuhkan karena
// owner & superuser SELALU bypass RLS, bahkan dengan FORCE. Harga dua koneksi
// itu bukan teknis melainkan manusiawi: satu env lagi yang bisa lupa diisi, satu
// password lagi yang bisa bocor, satu entri lagi di userlist.txt PgBouncer — dan
// bila DSN kedua diisi sama dengan yang pertama, semuanya lolos sambil tetap
// membocorkan data.
//
// Yang dijaga di sini adalah sifat-sifat yang membuat penggantian itu SAH.
// Kalau salah satunya rusak, penurunan hak berubah dari pengaman menjadi
// dekorasi — dan itu tak akan terlihat dari perilaku aplikasi.

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL tidak di-set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestWithTenant_MenurunkanHak: INTI perubahan. Koneksi dibuka sebagai owner
// (migrasi butuh ALTER/CREATE POLICY), tapi di dalam transaksi ia harus menjadi
// app_rw — dan atribut superuser ikut tercabut, sebab superuser lolos RLS tanpa
// syarat apa pun.
func TestWithTenant_MenurunkanHak(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	var user string
	var super bool
	err := WithTenant(ctx, pool, 1, func(q *Queries) error {
		return q.db.QueryRow(ctx,
			`SELECT current_user, (SELECT rolsuper FROM pg_roles WHERE rolname = current_user)`).
			Scan(&user, &super)
	})
	if err != nil {
		t.Fatalf("WithTenant: %v", err)
	}
	if user != runtimeRole {
		t.Errorf("transaksi harus berjalan sebagai %q, got %q", runtimeRole, user)
	}
	if super {
		t.Error("atribut superuser harus TERCABUT di dalam transaksi — superuser bypass RLS tanpa syarat")
	}
}

// TestWithSuper_TetapMenurunkanHak: jalur platform juga diturunkan haknya.
// Bypass-nya datang dari GUC app.is_super — keputusan yang diambil di Go — bukan
// dari privilege role. Kalau ia dibiarkan berjalan sebagai owner, jalur platform
// diam-diam jadi jalan pintas ke DDL.
func TestWithSuper_TetapMenurunkanHak(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	var user string
	if err := WithSuper(ctx, pool, func(q *Queries) error {
		return q.db.QueryRow(ctx, "SELECT current_user").Scan(&user)
	}); err != nil {
		t.Fatalf("WithSuper: %v", err)
	}
	if user != runtimeRole {
		t.Errorf("jalur platform pun harus berjalan sebagai %q, got %q", runtimeRole, user)
	}
}

// TestHakPulihSetelahTransaksi: yang membuat SET LOCAL aman di pool — dan yang
// paling mahal bila salah. Kalau hak bocor ke transaksi berikutnya di koneksi
// yang sama, migrasi berikutnya gagal dengan pesan yang tak menyebut sebabnya;
// kalau arahnya sebaliknya, satu tenant membaca baris tenant lain di bawah beban.
//
// Diuji untuk COMMIT dan ROLLBACK: jalur gagal justru yang paling jarang
// dilewati, jadi paling mungkin luput.
func TestHakPulihSetelahTransaksi(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	// Satu koneksi saja, supaya "transaksi berikutnya" benar-benar memakai
	// backend yang sama — kalau pool memberi koneksi lain, test ini lulus tanpa
	// membuktikan apa pun.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()

	baseline := func() string {
		var u string
		if err := conn.QueryRow(ctx, "SELECT current_user").Scan(&u); err != nil {
			t.Fatalf("baca current_user: %v", err)
		}
		return u
	}
	owner := baseline()
	if owner == runtimeRole {
		t.Skip("koneksi test sudah app_rw — pemulihan tak bisa dibedakan")
	}

	for _, akhir := range []string{"COMMIT", "ROLLBACK"} {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+runtimeRole); err != nil {
			t.Fatalf("set local role: %v", err)
		}
		var inside string
		if err := tx.QueryRow(ctx, "SELECT current_user").Scan(&inside); err != nil {
			t.Fatalf("baca di dalam tx: %v", err)
		}
		if inside != runtimeRole {
			t.Fatalf("di dalam tx harus %q, got %q", runtimeRole, inside)
		}
		if akhir == "COMMIT" {
			err = tx.Commit(ctx)
		} else {
			err = tx.Rollback(ctx)
		}
		if err != nil {
			t.Fatalf("%s: %v", akhir, err)
		}
		if got := baseline(); got != owner {
			t.Errorf("setelah %s hak harus pulih ke %q, got %q — ini kebocoran ke peminjam pool berikutnya",
				akhir, owner, got)
		}
	}
}

// TestRuntimeRoleTakBisaDDL: batas yang memisahkan runtime dari migrasi. Kalau
// app_rw bisa DDL, seluruh gunanya sebagai role terbatas hilang — dan yang
// menemukan biasanya adalah tabel yang tiba-tiba ada.
func TestRuntimeRoleTakBisaDDL(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	err := WithSuper(ctx, pool, func(q *Queries) error {
		_, e := q.db.Exec(ctx, "CREATE TABLE seharusnya_gagal (x int)")
		return e
	})
	if err == nil {
		t.Fatal("app_rw TAK BOLEH bisa CREATE TABLE")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "permission denied") {
		t.Errorf("harus ditolak karena privilege, got: %v", err)
	}
}

// TestRLSMengikatDiJalurAplikasi: pembuktian ujung-ke-ujung, dengan pertanyaan
// yang benar. Memeriksa pool telanjang akan menjawab "tidak mengikat" padahal
// setiap query aplikasi terikat — yang harus diukur adalah keadaan yang dijalani
// query sungguhan, yaitu DI DALAM transaksi yang sudah menurunkan haknya.
func TestRLSMengikatDiJalurAplikasi(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	var st RLSStatus
	if err := WithSuper(ctx, pool, func(q *Queries) error {
		var e error
		st, e = CheckRLSTx(ctx, q, "audit_logs")
		return e
	}); err != nil {
		t.Fatalf("CheckRLSTx: %v", err)
	}
	if !st.Binds() {
		t.Errorf("RLS harus MENGIKAT di jalur aplikasi: %s", st.Reason())
	}
	if !st.Forced {
		t.Error("FORCE RLS harus aktif — tanpanya pemilik tabel bypass policy diam-diam")
	}

	// Dan pemeriksaannya harus JUJUR: pool telanjang (owner) wajib dilaporkan
	// TIDAK mengikat. Kalau ia melaporkan aman di sini juga, ia hanya stempel.
	raw, err := CheckRLS(ctx, pool, "audit_logs")
	if err != nil {
		t.Fatalf("CheckRLS: %v", err)
	}
	if raw.User != runtimeRole && raw.Binds() {
		t.Errorf("koneksi %q (tanpa penurunan hak) tak boleh dilaporkan terikat RLS", raw.User)
	}
}
