package handler

import (
	"testing"

	"go_starter/internal/db"
	"go_starter/internal/oauth"
)

// oauth_profile_test.go — nama tampilan & avatar yang datang bersama login.
//
// Google mengirim `name` di setiap id_token, tapi selama ini nilainya dibuang
// karena tak ada tempat menyimpannya. Yang diuji di sini adalah rantai lengkap:
// claim → normalisasi → kolom `users.name`. Rantai ini punya tiga cabang
// (user baru, login berulang, auto-link akun lama) dan cabang yang tertinggal
// tak akan terlihat dari layar — orangnya cuma tampak "belum punya nama".

// nameOf membaca nama tersimpan seorang user; "" bila NULL.
func nameOf(t *testing.T, env *testEnv, email string) string {
	t.Helper()
	u, err := env.q.GetUserByEmail(t.Context(), email)
	if err != nil {
		t.Fatalf("ambil user %s: %v", email, err)
	}
	if u.Name == nil {
		return ""
	}
	return *u.Name
}

// TestGoogleCallback_UserBaruMenyimpanNama: cabang 3 (user baru sepenuhnya).
func TestGoogleCallback_UserBaruMenyimpanNama(t *testing.T) {
	env, _ := setupTest(t)
	SetGoogleOAuth(&stubProvider{claims: &oauth.Claims{
		Sub: "g-name-1", Email: "baru@gmail.com", EmailVerified: true,
		Name: "Budi Santoso",
	}})
	t.Cleanup(func() { SetGoogleOAuth(nil) })

	env.doCallback(t, "s1", "s1")
	if got := nameOf(t, env, "baru@gmail.com"); got != "Budi Santoso" {
		t.Errorf("nama dari Google tak tersimpan: got %q", got)
	}
}

// TestGoogleCallback_NamaWorkspaceTakTertukar: di cabang user-baru, nama ORANG
// dan nama WORKSPACE dirakit berdampingan. Keduanya pernah memakai identifier
// yang sama (`name`), dan penukaran seperti itu tak menghasilkan error apa pun —
// orangnya sekadar tersimpan dengan nama yang salah.
func TestGoogleCallback_NamaWorkspaceTakTertukar(t *testing.T) {
	env, _ := setupTest(t)
	SetGoogleOAuth(&stubProvider{claims: &oauth.Claims{
		Sub: "g-name-2", Email: "pemilik@gmail.com", EmailVerified: true,
		Name: "Budi Santoso",
	}})
	t.Cleanup(func() { SetGoogleOAuth(nil) })

	env.doCallback(t, "s1", "s1")
	// Nama workspace tetap diturunkan dari bagian email sebelum '@', BUKAN dari
	// nama orangnya.
	if got := nameOf(t, env, "pemilik@gmail.com"); got != "Budi Santoso" {
		t.Errorf("user harus menyimpan nama ORANG: got %q", got)
	}
	u, err := env.q.GetUserByEmail(t.Context(), "pemilik@gmail.com")
	if err != nil {
		t.Fatalf("ambil user: %v", err)
	}
	ms, err := env.q.ListMembershipsByUser(t.Context(), u.ID)
	if err != nil {
		t.Fatalf("list membership: %v", err)
	}
	if len(ms) == 0 {
		t.Fatal("user baru harus punya workspace")
	}
	if ms[0].Name == "Budi Santoso" {
		t.Error("workspace bernama sama dengan ORANGNYA — nama orang & nama workspace tertukar")
	}
}

// TestGoogleCallback_NamaDiRefreshSaatLoginUlang: cabang 1 (identitas sudah
// tertaut). Orang mengganti namanya di Google, dan login adalah satu-satunya
// saat kita mendengar kabarnya.
func TestGoogleCallback_NamaDiRefreshSaatLoginUlang(t *testing.T) {
	env, _ := setupTest(t)
	stub := &stubProvider{claims: &oauth.Claims{
		Sub: "g-name-3", Email: "ganti@gmail.com", EmailVerified: true,
		Name: "Nama Lama",
	}}
	SetGoogleOAuth(stub)
	t.Cleanup(func() { SetGoogleOAuth(nil) })

	env.doCallback(t, "s1", "s1")
	stub.claims.Name = "Nama Baru"
	env.doCallback(t, "s2", "s2")

	if got := nameOf(t, env, "ganti@gmail.com"); got != "Nama Baru" {
		t.Errorf("nama tak di-refresh saat login ulang: got %q", got)
	}
}

// TestGoogleCallback_ClaimKosongTakMenghapusProfil: Google BOLEH tak mengirim
// `name`/`picture` walau scope diminta. Menimpa lugas akan menghapus profil yang
// sudah benar — dan orangnya tak pernah melakukan apa pun untuk itu. COALESCE
// di SQL adalah yang menahannya; test ini yang menjaga COALESCE itu tetap ada.
func TestGoogleCallback_ClaimKosongTakMenghapusProfil(t *testing.T) {
	env, _ := setupTest(t)
	stub := &stubProvider{claims: &oauth.Claims{
		Sub: "g-name-4", Email: "tetap@gmail.com", EmailVerified: true,
		Name: "Budi Santoso", Picture: "https://lh3.googleusercontent.com/a/foto=s96-c",
	}}
	SetGoogleOAuth(stub)
	t.Cleanup(func() { SetGoogleOAuth(nil) })

	env.doCallback(t, "s1", "s1")
	// Login berikutnya: Google tak mengirim apa-apa soal profil.
	stub.claims.Name, stub.claims.Picture = "", ""
	env.doCallback(t, "s2", "s2")

	if got := nameOf(t, env, "tetap@gmail.com"); got != "Budi Santoso" {
		t.Errorf("nama terhapus oleh claim kosong: got %q", got)
	}
	u, err := env.q.GetUserByEmail(t.Context(), "tetap@gmail.com")
	if err != nil {
		t.Fatalf("ambil user: %v", err)
	}
	if u.AvatarUrl == nil {
		t.Error("avatar terhapus oleh claim kosong — nilai lama lebih baik daripada tak ada")
	}
}

// TestGoogleCallback_AutoLinkMenyimpanNama: cabang 2. Akun password lama yang
// belum pernah punya nama mendapatkannya saat pertama kali login lewat Google.
func TestGoogleCallback_AutoLinkMenyimpanNama(t *testing.T) {
	env, _ := setupTest(t)
	// Akun password dev yang sudah ada + punya workspace (syarat auto-link).
	u := env.seedUserOnly(t, "lama@gmail.com")
	if _, err := env.q.CreateMembership(t.Context(), db.CreateMembershipParams{
		UserID: u.ID, TenantID: env.tenantID, Role: "member",
	}); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	if nameOf(t, env, "lama@gmail.com") != "" {
		t.Fatal("prasyarat: akun password belum punya nama")
	}

	SetGoogleOAuth(&stubProvider{claims: &oauth.Claims{
		Sub: "g-name-5", Email: "lama@gmail.com", EmailVerified: true,
		Name: "Budi Santoso",
	}})
	t.Cleanup(func() { SetGoogleOAuth(nil) })

	env.doCallback(t, "s1", "s1")
	if got := nameOf(t, env, "lama@gmail.com"); got != "Budi Santoso" {
		t.Errorf("auto-link tak menyimpan nama: got %q", got)
	}
}

// TestGoogleCallback_NamaDibersihkan: normalisasi harus benar-benar terpasang di
// jalur login, bukan cuma ada sebagai fungsi yang lulus unit test sendirian.
func TestGoogleCallback_NamaDibersihkan(t *testing.T) {
	env, _ := setupTest(t)
	SetGoogleOAuth(&stubProvider{claims: &oauth.Claims{
		Sub: "g-name-6", Email: "kotor@gmail.com", EmailVerified: true,
		Name: "  Budi\nSantoso  ",
	}})
	t.Cleanup(func() { SetGoogleOAuth(nil) })

	env.doCallback(t, "s1", "s1")
	if got := nameOf(t, env, "kotor@gmail.com"); got != "Budi Santoso" {
		t.Errorf("nama tak dibersihkan di jalur login: got %q", got)
	}
}
