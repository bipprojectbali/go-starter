package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5/pgtype"
)

// notifications_test.go — notifikasi = umpan per-USER lintas-workspace.
//
// Tabel notifications SENGAJA TANPA RLS (lihat migrasi 00009), jadi isolasi
// antar-user bergantung SEPENUHNYA pada klausa WHERE di query. Test isolasi di
// file ini adalah pengganti jaring RLS — bukan pelengkap.

// mkNotif menyisipkan satu peristiwa langsung ke DB.
func (e *testEnv) mkNotif(t *testing.T, userID int64, kind string) {
	t.Helper()
	if _, err := e.q.CreateNotification(t.Context(), db.CreateNotificationParams{
		UserID: userID, TenantID: &e.tenantID, Kind: kind, Payload: []byte(`{}`),
	}); err != nil {
		t.Fatalf("seed notifikasi: %v", err)
	}
}

// firstPage membaca halaman pertama notifikasi milik satu user.
func (e *testEnv) firstPage(t *testing.T, userID int64) []db.ListNotificationsRow {
	t.Helper()
	at, id := firstPageCursor()
	rows, err := e.q.ListNotifications(t.Context(), db.ListNotificationsParams{
		UserID: userID, CursorCreatedAt: at, CursorID: id, PageSize: notifPageSize,
	})
	if err != nil {
		t.Fatalf("list notifikasi: %v", err)
	}
	return rows
}

// TestNotif_TakBocorAntarUser: INI pengganti RLS. User hanya melihat
// notifikasinya sendiri — dijamin oleh WHERE user_id, bukan oleh policy DB.
func TestNotif_TakBocorAntarUser(t *testing.T) {
	env, uidA := setupTest(t)
	b := env.seedMember(t, "b@local", "member", 0)

	env.mkNotif(t, uidA, "member.role.changed")
	env.mkNotif(t, b.ID, "member.removed")
	env.mkNotif(t, b.ID, "member.role.changed")

	rowsA := env.firstPage(t, uidA)
	if len(rowsA) != 1 {
		t.Fatalf("A harus lihat 1 notifikasi miliknya, got %d", len(rowsA))
	}
	for _, r := range rowsA {
		if r.UserID != uidA {
			t.Errorf("BOCOR: notifikasi user %d terbaca oleh user %d", r.UserID, uidA)
		}
	}
	if got := len(env.firstPage(t, b.ID)); got != 2 {
		t.Errorf("B harus lihat 2 notifikasi miliknya, got %d", got)
	}
}

// TestNotif_UndanganDicocokkanPerEmailCaseInsensitive: penerima diundang dengan
// kapitalisasi berbeda dari email akunnya — tetap harus ketemu, karena orang
// mengetik email seenaknya saat mengundang.
func TestNotif_UndanganDicocokkanPerEmailCaseInsensitive(t *testing.T) {
	env, _ := setupTest(t)
	env.mkInvite(t, "tok-case", "Tamu@Local", "member", time.Hour)

	got, err := env.q.ListPendingInvitesByEmail(t.Context(), "tamu@local")
	if err != nil {
		t.Fatalf("list undangan: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("undangan harus ketemu walau beda kapitalisasi, got %d", len(got))
	}
	if got[0].TenantName != "Test" {
		t.Errorf("nama workspace harus ikut (untuk ditampilkan), got %q", got[0].TenantName)
	}
}

// TestNotif_UndanganKedaluwarsaTakMuncul: yang lewat masa berlaku bukan lagi
// tugas — tak boleh menghantui inbox maupun badge.
func TestNotif_UndanganKedaluwarsaTakMuncul(t *testing.T) {
	env, _ := setupTest(t)
	env.mkInvite(t, "tok-hidup", "x@local", "member", time.Hour)
	env.mkInvite(t, "tok-mati", "x@local", "member", -time.Hour)

	got, err := env.q.ListPendingInvitesByEmail(t.Context(), "x@local")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].Token != "tok-hidup" {
		t.Errorf("hanya undangan yang masih berlaku yang muncul, got %d", len(got))
	}
	n, err := env.q.CountPendingInvitesByEmail(t.Context(), "x@local")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("badge harus menghitung 1 undangan hidup, got %d", n)
	}
}

// TestNotif_AutoReadTapiUndanganTetapTerhitung: inti keputusan desain. Membuka
// halaman menandai PERISTIWA terbaca, tapi undangan pending tetap masuk badge —
// kalau ikut ternol, user bisa lupa menindak dan undangan lenyap senyap.
func TestNotif_AutoReadTapiUndanganTetapTerhitung(t *testing.T) {
	env, uid := setupTest(t)
	env.mkNotif(t, uid, "member.role.changed")
	env.mkInvite(t, "tok-pending", "test@local", "member", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/notifications", nil)
	rec := env.doAuthed(uid, req, func(w http.ResponseWriter, r *http.Request) {
		session.SetIdentity(r.Context(), uid, "test@local", "owner", false,
			env.tenantID, "Test", "")
		env.h.NotificationsPage(w, r)
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("halaman notifikasi status %d", rec.Code)
	}
	body := rec.Body.String()
	if !contains(body, "tok-pending") {
		t.Error("undangan pending harus tampil di halaman (form terima/tolak)")
	}

	unread, err := env.q.CountUnreadNotifications(t.Context(), uid)
	if err != nil {
		t.Fatalf("count unread: %v", err)
	}
	if unread != 0 {
		t.Errorf("peristiwa harus ditandai terbaca setelah halaman dibuka, sisa %d", unread)
	}
	inv, err := env.q.CountPendingInvitesByEmail(t.Context(), "test@local")
	if err != nil {
		t.Fatalf("count invite: %v", err)
	}
	if inv != 1 {
		t.Errorf("undangan pending TAK BOLEH ikut ter-auto-read, got %d", inv)
	}
}

// TestNotif_TolakUndanganTerkunciEmail: pemegang token tak bisa menolak undangan
// milik orang lain (DeclineInvite mengunci token DAN email).
func TestNotif_TolakUndanganTerkunciEmail(t *testing.T) {
	env, _ := setupTest(t)
	env.mkInvite(t, "tok-milik-a", "a@local", "member", time.Hour)

	// Orang lain (b@local) memegang token tapi bukan penerimanya.
	if err := env.q.DeclineInvite(t.Context(), db.DeclineInviteParams{
		Token: "tok-milik-a", Email: "b@local",
	}); err != nil {
		t.Fatalf("decline: %v", err)
	}
	if n, _ := env.q.CountPendingInvitesByEmail(t.Context(), "a@local"); n != 1 {
		t.Error("undangan milik A tak boleh terhapus oleh pemegang token lain")
	}

	// Penerima yang sah menolak → hilang.
	if err := env.q.DeclineInvite(t.Context(), db.DeclineInviteParams{
		Token: "tok-milik-a", Email: "a@local",
	}); err != nil {
		t.Fatalf("decline sah: %v", err)
	}
	if n, _ := env.q.CountPendingInvitesByEmail(t.Context(), "a@local"); n != 0 {
		t.Error("penerima sah harus bisa menolak undangannya")
	}
}

// TestNotif_KeysetPaginasi: daftar dibatasi & cursor memajukan halaman (Rule 13).
func TestNotif_KeysetPaginasi(t *testing.T) {
	env, uid := setupTest(t)
	for i := 0; i < 3; i++ {
		env.mkNotif(t, uid, "member.role.changed")
	}
	rows, err := env.q.ListNotifications(t.Context(), db.ListNotificationsParams{
		UserID: uid, CursorCreatedAt: pgtype.Timestamptz{Valid: true, InfinityModifier: pgtype.Infinity},
		CursorID: 1 << 62, PageSize: 2,
	})
	if err != nil {
		t.Fatalf("halaman 1: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("page_size harus dihormati, got %d", len(rows))
	}
	next, err := env.q.ListNotifications(t.Context(), db.ListNotificationsParams{
		UserID: uid, CursorCreatedAt: rows[1].CreatedAt, CursorID: rows[1].ID, PageSize: 2,
	})
	if err != nil {
		t.Fatalf("halaman 2: %v", err)
	}
	if len(next) != 1 {
		t.Errorf("halaman 2 harus berisi sisa 1 baris, got %d", len(next))
	}
	for _, r := range next {
		if r.ID == rows[0].ID || r.ID == rows[1].ID {
			t.Error("cursor tak boleh mengulang baris halaman sebelumnya")
		}
	}
}

// TestNotif_UbahRoleMemberiTahuYangBersangkutan: aksi admin memicu notifikasi ke
// TARGET (bukan aktor), dengan role baru tercatat di payload.
func TestNotif_UbahRoleMemberiTahuYangBersangkutan(t *testing.T) {
	env, ownerID := setupTest(t)
	target := env.seedMember(t, "target@local", "member", 0)

	env.withSession(t, ownerID, func(sc sessionCtx) {
		env.h.notify(sc.ctx, target.ID, env.tenantID, "member.role.changed",
			notifPayload{Role: "admin"})
	})

	rows := env.firstPage(t, target.ID)
	if len(rows) != 1 {
		t.Fatalf("target harus menerima 1 notifikasi, got %d", len(rows))
	}
	if rows[0].Kind != "member.role.changed" {
		t.Errorf("kind salah: %q", rows[0].Kind)
	}
	view := buildNotifRows(rows)
	if len(view) != 1 || !contains(view[0].Text, "admin") {
		t.Errorf("kalimat harus menyebut role baru, got %+v", view)
	}
	// Aktor TIDAK ikut diberi tahu atas aksinya sendiri.
	if got := len(env.firstPage(t, ownerID)); got != 0 {
		t.Errorf("aktor tak perlu notifikasi atas aksinya sendiri, got %d", got)
	}
}

// contains = alias tipis strings.Contains agar assertion terbaca sebagai niat
// ("body memuat token"), bukan detail pustaka.
func contains(s, sub string) bool { return strings.Contains(s, sub) }
