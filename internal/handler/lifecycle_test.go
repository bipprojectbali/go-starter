package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5/pgtype"
)

// lifecycle_test.go — keputusan 0005. Yang dijaga di sini bukan sekadar "query
// jalan", melainkan tiga hal yang mahal bila salah: workspace mati tak bisa
// diakses, kewenangan suspend/archive tak bisa saling menembus, dan bukti audit
// selamat dari penghapusan.

// setStatus mengubah status/deleted_at langsung lewat SQL — memotong handler
// agar yang diuji adalah GERBANGNYA, bukan jalur yang membuat keadaan itu.
func setStatus(t *testing.T, env *testEnv, tenantID int64, status string) {
	t.Helper()
	ctx := t.Context()
	switch status {
	case TenantSuspended:
		by, reason := int64(1), "tunggakan pembayaran"
		if err := env.q.SuspendTenant(ctx, db.SuspendTenantParams{
			ID: tenantID, SuspendedBy: &by, SuspendReason: &reason,
		}); err != nil {
			t.Fatalf("suspend: %v", err)
		}
	case TenantArchived:
		if err := env.q.ArchiveTenant(ctx, tenantID); err != nil {
			t.Fatalf("archive: %v", err)
		}
	}
}

// gate menjalankan gateLifecycle terhadap tenant terkini + metode HTTP tertentu,
// mengembalikan (lolos, status code yang ditulis).
//
// Dijalankan DI DALAM session: halaman penjelasan suspensi dirender lewat
// renderPage, yang membaca email/avatar dari session — di produksi Scope selalu
// berjalan setelah middleware session, jadi test harus meniru urutan itu.
func gate(t *testing.T, env *testEnv, tenantID int64, method string) (bool, int) {
	t.Helper()
	tn, err := env.q.GetTenant(t.Context(), tenantID)
	if err != nil {
		t.Fatalf("get tenant: %v", err)
	}
	var (
		ok   bool
		code int
	)
	env.withSession(t, 1, func(sc sessionCtx) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/w/test/members", nil).WithContext(sc.ctx)
		ok = env.h.gateLifecycle(rec, req, tn)
		code = rec.Code
	})
	return ok, code
}

// TestGate_ActiveLolos: jalur normal tak boleh ikut terhalang oleh gerbang baru.
func TestGate_ActiveLolos(t *testing.T) {
	env, _ := setupTest(t)
	for _, m := range []string{http.MethodGet, http.MethodPost} {
		if ok, _ := gate(t, env, env.tenantID, m); !ok {
			t.Errorf("workspace aktif harus lolos untuk %s", m)
		}
	}
}

// TestGate_Suspended403BukanA404: anggota SAH berhak tahu KENAPA ia tak bisa
// masuk. 404 akan membuatnya mengira workspace-nya hilang lalu menghubungi
// support tanpa perlu — yang dilindungi 0004 adalah keberadaan workspace dari
// ORANG LUAR, bukan alasan dari orang dalam.
func TestGate_Suspended403BukanA404(t *testing.T) {
	env, _ := setupTest(t)
	setStatus(t, env, env.tenantID, TenantSuspended)

	ok, code := gate(t, env, env.tenantID, http.MethodGet)
	if ok {
		t.Fatal("workspace ditangguhkan tak boleh lolos")
	}
	if code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (bukan 404: anggota berhak tahu alasannya)", code)
	}
}

// TestGate_ArchivedBacaBolehTulisTolak: read-only ditegakkan per-METODE, bukan
// dengan menyembunyikan tombol. Form yang di-POST langsung harus tetap ditolak.
func TestGate_ArchivedBacaBolehTulisTolak(t *testing.T) {
	env, _ := setupTest(t)
	setStatus(t, env, env.tenantID, TenantArchived)

	if ok, _ := gate(t, env, env.tenantID, http.MethodGet); !ok {
		t.Error("workspace terarsip harus tetap bisa DIBACA")
	}
	ok, code := gate(t, env, env.tenantID, http.MethodPost)
	if ok {
		t.Fatal("workspace terarsip harus MENOLAK perubahan")
	}
	if code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", code)
	}
}

// TestGate_Deleted404: bagi anggota, workspace terhapus memang sudah berakhir.
// Masa tenggang adalah jaring operasional (restore lewat panel), bukan keadaan
// yang perlu dijelaskan ke mereka.
func TestGate_Deleted404(t *testing.T) {
	env, _ := setupTest(t)
	if err := env.q.SoftDeleteTenant(t.Context(), env.tenantID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	ok, code := gate(t, env, env.tenantID, http.MethodGet)
	if ok {
		t.Fatal("workspace terhapus tak boleh lolos")
	}
	if code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", code)
	}
}

// TestPlatformTembusSuspensi: platform TETAP bisa masuk workspace yang
// ditangguhkan — merekalah yang menangguhkan, jadi menghalangi mereka membuat
// suspensi mustahil diselidiki dan restore mustahil diverifikasi sebelum
// ditekan. Cabang platform di Scope sengaja tak memanggil gateLifecycle; test
// ini yang membuatnya keputusan, bukan kelalaian yang kebetulan bekerja.
func TestPlatformTembusSuspensi(t *testing.T) {
	env, uid := setupTest(t)
	setStatus(t, env, env.tenantID, TenantSuspended)

	env.withSession(t, uid, func(sc sessionCtx) {
		session.SetIdentity(sc.ctx, uid, "root@local", "super_admin", true,
			env.tenantID, "Test", "test", "")
		// adoptTenantBySlug = jalur platform. Ia HANYA memeriksa keberadaan slug,
		// tak peduli status — itulah yang membuat penyelidikan mungkin.
		if _, ok := env.h.adoptTenantBySlug(sc.ctx, "test"); !ok {
			t.Error("platform harus tetap bisa membuka workspace yang ditangguhkan")
		}
	})
}

// TestUnarchive_PlatformBukanAnggota: REGRESI (ditemukan di browser). Route
// unarchive berada di luar Scope, jadi ia mencari tenant sendiri. Sempat memakai
// resolveTenantBySlug yang MENSYARATKAN keanggotaan — sementara platform bukan
// anggota workspace mana pun, sehingga ia ditolak 404 di sini padahal isOwnerOf
// di baris berikutnya justru mengizinkannya. Dua cek yang saling bertentangan.
func TestUnarchive_PlatformBukanAnggota(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()

	// Workspace milik ORANG LAIN, lalu diarsipkan.
	asing, err := env.q.CreateTenant(ctx, db.CreateTenantParams{Name: "Asing", Slug: "asing"})
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	env.seedMember(t, "orang2@local", "owner", asing.ID)
	if err := env.q.ArchiveTenant(ctx, asing.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}

	env.withSession(t, uid, func(sc sessionCtx) {
		session.SetIdentity(sc.ctx, uid, "root@local", "super_admin", true,
			env.tenantID, "Test", "test", "")
		// Jalur pencarian platform: TANPA syarat keanggotaan.
		tn, ok := env.h.tenantBySlug(sc.ctx, "asing")
		if !ok {
			t.Fatal("platform harus menemukan workspace yang bukan miliknya")
		}
		if !env.h.isOwnerOf(sc.ctx, uid, tn.ID) {
			t.Error("platform harus berwenang memulihkan workspace mana pun")
		}
		// Jalur anggota biasa tetap menolak — pemisahan tak boleh melonggarkan ini.
		session.SetIdentity(sc.ctx, uid, "biasa@local", "member", false,
			env.tenantID, "Test", "test", "")
		if _, ok := env.h.resolveTenantBySlug(sc.ctx, uid, "asing"); ok {
			t.Error("KEBOCORAN: non-anggota tak boleh menemukan workspace orang lain")
		}
	})
}

// TestArchive_TakBisaMenimpaSuspensi: INTI PEMISAHAN KEWENANGAN (0005 §1).
// Kalau owner bisa mengarsipkan workspace yang ditangguhkan lalu mengaktifkannya
// kembali, ia keluar dari suspensi platform lewat pintu samping — dan suspensi
// jadi tak berarti apa-apa.
func TestArchive_TakBisaMenimpaSuspensi(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	setStatus(t, env, env.tenantID, TenantSuspended)

	if err := env.q.ArchiveTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("archive (harusnya no-op): %v", err)
	}
	tn, _ := env.q.GetTenant(ctx, env.tenantID)
	if tn.Status != TenantSuspended {
		t.Fatalf("status = %q — archive TAK BOLEH menimpa suspensi platform", tn.Status)
	}
	// Dan unarchive pun tak boleh jadi jalan keluar.
	if err := env.q.UnarchiveTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("unarchive: %v", err)
	}
	tn, _ = env.q.GetTenant(ctx, env.tenantID)
	if tn.Status != TenantSuspended {
		t.Errorf("status = %q — unarchive tak boleh membatalkan suspensi", tn.Status)
	}
}

// TestUnsuspend_MemulihkanAkses: suspensi harus benar-benar bisa dicabut, dan
// jejaknya dibersihkan agar tak jadi sisa yang menyesatkan di suspensi berikutnya.
func TestUnsuspend_MemulihkanAkses(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	setStatus(t, env, env.tenantID, TenantSuspended)

	if err := env.q.UnsuspendTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("unsuspend: %v", err)
	}
	tn, _ := env.q.GetTenant(ctx, env.tenantID)
	if tn.Status != TenantActive {
		t.Fatalf("status = %q, want active", tn.Status)
	}
	if tn.SuspendReason != nil || tn.SuspendedAt.Valid {
		t.Error("jejak suspensi harus dibersihkan, bukan ditinggal sebagai sisa")
	}
	if ok, _ := gate(t, env, env.tenantID, http.MethodPost); !ok {
		t.Error("setelah unsuspend, perubahan harus diizinkan lagi")
	}
}

// TestSoftDelete_HilangDariSwitcherDanKuota: workspace terhapus tak boleh muncul
// di switcher (0005) — ia juga sumber pilihan fallback Scope, jadi kalau tampak,
// user bisa dilempar ke workspace yang sudah dihapus. Kuota pun tak boleh
// tertahan olehnya: itu terasa seperti bug & mendorong purge lebih cepat.
func TestSoftDelete_HilangDariSwitcherDanKuota(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()

	before, _ := env.q.CountOwnedWorkspaces(ctx, uid)
	if before != 1 {
		t.Fatalf("awal harus 1 workspace dimiliki, got %d", before)
	}
	if err := env.q.SoftDeleteTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	ms, err := env.q.ListMembershipsByUser(ctx, uid)
	if err != nil {
		t.Fatalf("list memberships: %v", err)
	}
	if len(ms) != 0 {
		t.Errorf("workspace terhapus masih muncul di switcher: %+v", ms)
	}
	after, _ := env.q.CountOwnedWorkspaces(ctx, uid)
	if after != 0 {
		t.Errorf("kuota masih tertahan workspace terhapus: %d", after)
	}
}

// TestArchived_TetapMemakanKuota: kebalikan dari terhapus — datanya masih
// disimpan & bisa diaktifkan kapan saja, jadi arsip tak boleh jadi celah kuota
// gratis (0005 §7).
func TestArchived_TetapMemakanKuota(t *testing.T) {
	env, uid := setupTest(t)
	setStatus(t, env, env.tenantID, TenantArchived)

	n, err := env.q.CountOwnedWorkspaces(t.Context(), uid)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("workspace terarsip HARUS tetap memakan kuota, got %d", n)
	}
}

// TestRestore_MengembalikanKeadaanTerpakai: pemulihan harus meninggalkan
// workspace yang bisa langsung dipakai, bukan setengah jalan (mis. kembali
// sebagai terarsip sehingga owner harus menekan tombol kedua).
func TestRestore_MengembalikanKeadaanTerpakai(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()
	setStatus(t, env, env.tenantID, TenantArchived)
	if err := env.q.SoftDeleteTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if err := env.q.RestoreTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("restore: %v", err)
	}
	tn, _ := env.q.GetTenant(ctx, env.tenantID)
	if tn.DeletedAt.Valid {
		t.Error("restore harus membatalkan penghapusan")
	}
	if tn.Status != TenantActive {
		t.Errorf("status setelah restore = %q, want active (siap pakai)", tn.Status)
	}
	ms, _ := env.q.ListMembershipsByUser(ctx, uid)
	if len(ms) != 1 {
		t.Errorf("workspace harus kembali ke switcher, got %d", len(ms))
	}
}

// TestPurge_AuditSelamat: BUKTI TAK BOLEH LENYAP BERSAMA YANG DIBUKTIKAN.
// memberships ikut CASCADE (data operasional), audit_logs TIDAK — FK-nya
// ON DELETE SET NULL sejak migrasi 00010. Justru pada peristiwa terpenting
// (penghapusan workspace) jejaknya paling dibutuhkan.
func TestPurge_AuditSelamat(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()

	if _, err := env.q.CreateAuditLog(ctx, db.CreateAuditLogParams{
		ActorUserID: &uid, Action: "workspace.delete", TargetType: "tenant",
		TargetID: &env.tenantID, Metadata: []byte("{}"), TenantID: &env.tenantID,
	}); err != nil {
		t.Fatalf("seed audit: %v", err)
	}
	if err := env.q.SoftDeleteTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if err := env.q.PurgeTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("purge GAGAL — FK audit memblokir penghapusan: %v", err)
	}
	// Tenant benar-benar hilang…
	if _, err := env.q.GetTenant(ctx, env.tenantID); err == nil {
		t.Error("tenant harus terhapus permanen setelah purge")
	}
	// …tapi jejaknya tidak.
	var n int64
	if err := env.h.Pool.QueryRow(ctx,
		"SELECT count(*) FROM audit_logs WHERE action = 'workspace.delete'").Scan(&n); err != nil {
		t.Fatalf("hitung audit: %v", err)
	}
	if n != 1 {
		t.Errorf("audit log harus SELAMAT dari purge, got %d baris", n)
	}
}

// TestListExpired_HanyaYangLewatTenggang: purge hanya menyentuh yang sudah
// melewati masa tenggang. Kalau tidak, "soft delete" cuma jeda semu.
func TestListExpired_HanyaYangLewatTenggang(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	if err := env.q.SoftDeleteTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	cutoff := time.Now().Add(-time.Duration(GracePeriodDays) * 24 * time.Hour)

	rows, err := env.q.ListExpiredTenants(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
	if err != nil {
		t.Fatalf("list expired: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("workspace yang baru dihapus BELUM boleh dipurge, got %d", len(rows))
	}
	// Yang dihapus jauh di masa lalu → kandidat sah.
	if _, err := env.h.Pool.Exec(ctx,
		"UPDATE tenants SET deleted_at = now() - interval '60 days' WHERE id = $1", env.tenantID); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	rows, err = env.q.ListExpiredTenants(ctx, pgtype.Timestamptz{Time: cutoff, Valid: true})
	if err != nil {
		t.Fatalf("list expired (2): %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("workspace lewat tenggang harus jadi kandidat purge, got %d", len(rows))
	}
}

// TestSlugTakDilepasSaatTerhapus: slug tetap dipesan selama masa tenggang —
// kalau dilepas, orang lain bisa mengambilnya dan restore jadi mustahil.
func TestSlugTakDilepasSaatTerhapus(t *testing.T) {
	env, _ := setupTest(t)
	ctx := t.Context()
	if err := env.q.SoftDeleteTenant(ctx, env.tenantID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	taken, err := env.q.TenantSlugExists(ctx, "test")
	if err != nil {
		t.Fatalf("slug exists: %v", err)
	}
	if !taken {
		t.Error("slug workspace terhapus HARUS tetap terpesan (restore jadi mustahil bila diambil orang)")
	}
}

// TestScopeSlug_TerhapusDitolak: gerbang di Scope, bukan cuma di query. Anggota
// yang membuka slug workspace terhapus tak boleh menembus ke dalamnya.
func TestScopeSlug_TerhapusDitolak(t *testing.T) {
	env, uid := setupTest(t)
	if err := env.q.SoftDeleteTenant(t.Context(), env.tenantID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	env.withSession(t, uid, func(sc sessionCtx) {
		// resolveTenantBySlug tetap MENEMUKAN barisnya (jalur restore platform
		// membutuhkannya) — yang menolak adalah gateLifecycle.
		tn, ok := env.h.resolveTenantBySlug(sc.ctx, uid, "test")
		if !ok {
			t.Fatal("baris tenant harus tetap terbaca untuk jalur restore")
		}
		if !tn.DeletedAt.Valid {
			t.Fatal("tenant harus bertanda terhapus")
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/w/test", nil).WithContext(sc.ctx)
		if env.h.gateLifecycle(rec, req, tn) {
			t.Error("workspace terhapus tak boleh lolos gerbang")
		}
		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}
