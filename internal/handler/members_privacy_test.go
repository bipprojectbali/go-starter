package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go_starter/internal/appmode"
	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5/pgtype"
)

// members_privacy_test.go — siapa melihat email siapa di halaman anggota.
//
// Halaman ini SENGAJA terbuka untuk semua anggota (0004: beda role = beda AKSI,
// bukan beda ALAMAT) — tahu siapa yang punya akses adalah bagian dari
// mempercayai ruang bersama. Yang dibatasi adalah PII-nya.
//
// Yang dijaga di sini adalah hal yang mustahil dilihat dari layar: email asli
// tak boleh SAMPAI ke browser yang tak berhak. Menyembunyikannya lewat CSS atau
// menyamarkannya di view tetap mengirimkannya, dan di sana ia terbaca di
// view-source meski tak tampak.

// renderMembers merender halaman anggota sebagai role tertentu dan mengembalikan
// HTML mentah yang benar-benar dikirim ke browser.
func renderMembers(t *testing.T, env *testEnv, uid int64, role string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/w/test/members", nil)
	rec := httptest.NewRecorder()
	env.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.SetIdentity(r.Context(), uid, "pelihat@local", role, false,
			env.tenantID, "Test", "test", "")
		env.h.MembersPage(w, r.WithContext(withQueries(r.Context(), env.q)))
	})).ServeHTTP(rec, req)
	return rec.Body.String()
}

// seedMember menambah satu anggota ke workspace seed dan mengembalikan emailnya.
func seedMember(t *testing.T, env *testEnv, email, role string) string {
	t.Helper()
	u := env.seedUserOnly(t, email)
	if _, err := env.q.CreateMembership(t.Context(), db.CreateMembershipParams{
		UserID: u.ID, TenantID: env.tenantID, Role: role,
	}); err != nil {
		t.Fatalf("seed anggota %s: %v", email, err)
	}
	return email
}

// TestMembers_EmailDisamarkanBagiMember: INTI opsi B. Email asli tak boleh ada
// di HTML sama sekali — bukan sekadar tak terlihat.
func TestMembers_EmailDisamarkanBagiMember(t *testing.T) {
	env, uid := setupTest(t)
	rekan := seedMember(t, env, "rekankerja@contoh.com", authz.RoleNameMember)

	withMode(t, appmode.Multi, func() {
		html := renderMembers(t, env, uid, authz.RoleNameMember)

		if strings.Contains(html, rekan) {
			t.Errorf("email ASLI rekan terkirim ke browser member — penyamaran gagal")
		}
		if !strings.Contains(html, "rek•••@contoh.com") {
			t.Error("bentuk samaran harus tetap tampil (barisnya masih perlu dibedakan)")
		}
		// Domain dipertahankan — itulah yang membedakan rekan satu organisasi dari
		// orang luar, dan justru pertanyaan itu yang membuat daftar ini berguna.
		if !strings.Contains(html, "@contoh.com") {
			t.Error("domain harus tetap terbaca")
		}
		// Penyamaran WAJIB dijelaskan; tanpa keterangan ia terbaca seperti data rusak.
		if !strings.Contains(html, "disamarkan") {
			t.Error("harus ada keterangan bahwa email disamarkan")
		}
	})
}

// TestMembers_EmailSendiriSelaluUtuh: menyamarkan email seseorang dari DIRINYA
// SENDIRI hanya membuatnya mengira sedang melihat baris orang lain.
func TestMembers_EmailSendiriSelaluUtuh(t *testing.T) {
	env, uid := setupTest(t)
	// setupTest menyeed user "test@local" sebagai owner; jadikan ia member biasa
	// supaya yang diuji adalah jalur non-pengelola.
	if err := env.q.UpdateMemberRole(t.Context(), db.UpdateMemberRoleParams{
		UserID: uid, TenantID: env.tenantID, Role: authz.RoleNameMember,
	}); err != nil {
		t.Fatalf("turunkan role: %v", err)
	}

	withMode(t, appmode.Multi, func() {
		html := renderMembers(t, env, uid, authz.RoleNameMember)
		if !strings.Contains(html, "test@local") {
			t.Error("email SENDIRI harus utuh — menyamarkannya dari diri sendiri membingungkan")
		}
	})
}

// TestMembers_PengelolaMelihatUtuh: sisi lain. Owner & admin MEMBUTUHKAN alamat
// lengkap (mengundang, mencocokkan orang), jadi penyamaran tak boleh mengenai
// mereka — kalau iya, fitur kelola anggota jadi mustahil dipakai.
func TestMembers_PengelolaMelihatUtuh(t *testing.T) {
	env, uid := setupTest(t)
	rekan := seedMember(t, env, "rekankerja@contoh.com", authz.RoleNameMember)

	withMode(t, appmode.Multi, func() {
		for _, role := range []string{authz.RoleNameOwner, authz.RoleNameAdmin} {
			html := renderMembers(t, env, uid, role)
			if !strings.Contains(html, rekan) {
				t.Errorf("%s harus melihat email utuh — ia yang mengelola anggota", role)
			}
			// Keterangan penyamaran tak boleh muncul bagi yang melihat utuh.
			if strings.Contains(html, "disamarkan") {
				t.Errorf("%s tak boleh melihat keterangan penyamaran", role)
			}
		}
	})
}

// TestMembers_UndanganTakBocorKeMember: daftar undangan pending memuat email
// ORANG YANG BELUM BERGABUNG plus TAUTAN BERTOKEN. Token itu setara kredensial —
// siapa pun yang memegangnya bisa masuk sebagai penerima undangan.
func TestMembers_UndanganTakBocorKeMember(t *testing.T) {
	env, uid := setupTest(t)
	ctx := t.Context()
	if _, err := env.q.CreateInvite(ctx, db.CreateInviteParams{
		TenantID: env.tenantID, Email: "calon@contoh.com",
		Role: authz.RoleNameMember, Token: "token-rahasia-uji",
		InvitedBy: &uid,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(inviteTTL), Valid: true},
	}); err != nil {
		t.Fatalf("seed undangan: %v", err)
	}

	withMode(t, appmode.Multi, func() {
		html := renderMembers(t, env, uid, authz.RoleNameMember)
		if strings.Contains(html, "token-rahasia-uji") {
			t.Error("token undangan TAK BOLEH sampai ke member — ia setara kredensial")
		}
		if strings.Contains(html, "calon@contoh.com") {
			t.Error("email calon anggota tak boleh terlihat member")
		}
	})
}
