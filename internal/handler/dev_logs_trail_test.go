package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go_starter/internal/session"
)

// dev_logs_trail_test.go — jejak aktivitas di panel /dev/logs.
//
// Yang digantikan: tabel yang hanya menampilkan login/logout, mengabaikan
// rentang yang dipilih, dan berhenti di 20 baris tanpa jalan keluar. Ketiganya
// gagal SENYAP — halamannya tampil rapi, isinya saja yang tak lengkap — jadi
// hanya test yang bisa menahannya kembali.

// seedAudit menyisipkan satu baris audit dengan created_at eksplisit, supaya
// test bisa menempatkan peristiwa di dalam & di luar rentang.
func seedAudit(t *testing.T, env *testEnv, actorID int64, action, targetType string,
	targetID int64, meta string, at time.Time) {
	t.Helper()
	if meta == "" {
		meta = "{}"
	}
	_, err := env.h.Pool.Exec(t.Context(),
		`INSERT INTO audit_logs (actor_user_id, action, target_type, target_id, metadata, tenant_id, created_at)
		 VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)`,
		actorID, action, targetType, targetID, meta, env.tenantID, at)
	if err != nil {
		t.Fatalf("seed audit %s: %v", action, err)
	}
}

// getLogs memanggil panel aktivitas sebagai super-admin dan mengembalikan HTML.
func getLogs(t *testing.T, env *testEnv, uid int64, query string) string {
	t.Helper()
	url := "/dev/logs"
	if query != "" {
		url += "?" + query
	}
	rec := httptest.NewRecorder()
	env.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.SetIdentity(r.Context(), uid, "test@local", "super_admin", true,
			env.tenantID, "Test", "test", "")
		env.h.DevLogs(w, r.WithContext(withQueries(r.Context(), env.q)))
	})).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec.Body.String()
}

// TestTrail_MenampilkanLebihDariLoginLogout: INTI perbaikan. Sebelumnya query
// menyaring `action IN ('auth.login','auth.logout')`, sehingga 14 jenis aksi
// lain tercatat di DB tapi tak pernah terlihat siapa pun.
func TestTrail_MenampilkanLebihDariLoginLogout(t *testing.T) {
	env, uid := setupTest(t)
	now := time.Now()
	seedAudit(t, env, uid, "auth.login", "session", uid, `{"method":"google"}`, now.Add(-time.Minute))
	seedAudit(t, env, uid, "workspace.rename", "workspace", env.tenantID, "", now.Add(-2*time.Minute))
	seedAudit(t, env, uid, "member.role.update", "user", uid, `{"to":"admin"}`, now.Add(-4*time.Minute))

	html := getLogs(t, env, uid, "")

	for _, want := range []string{"workspace.rename", "member.role.update", "auth.login"} {
		if !strings.Contains(html, want) {
			t.Errorf("aksi %q tak muncul di jejak — datanya tercatat tapi tak terlihat", want)
		}
	}
	// Kalimatnya, bukan cuma kodenya: "#8" menuntut pembacanya mencari sendiri
	// siapa itu, tepat saat ia sedang menyelidiki sesuatu.
	if !strings.Contains(html, "menjadi admin") {
		t.Errorf("peristiwa harus dirangkai jadi kalimat yang terbaca:\n%s", firstLines(html))
	}
}

// TestTrail_MenghormatiRentang: tab rentang dulu tak berpengaruh sama sekali
// pada tabel ini — memilih "Bulanan" tetap menampilkan 20 terbaru, jadi tab-nya
// berbohong.
func TestTrail_MenghormatiRentang(t *testing.T) {
	env, uid := setupTest(t)
	now := time.Now()
	seedAudit(t, env, uid, "workspace.create", "workspace", env.tenantID, "", now.Add(-5*time.Minute))
	// 10 hari lalu: DI LUAR rentang harian & mingguan, DI DALAM bulanan.
	seedAudit(t, env, uid, "workspace.suspend", "workspace", env.tenantID,
		`{"reason":"tunggakan"}`, now.AddDate(0, 0, -10))

	harian := getLogs(t, env, uid, "range=day")
	if strings.Contains(harian, "workspace.suspend") {
		t.Error("peristiwa 10 hari lalu tak boleh muncul di rentang harian")
	}
	if !strings.Contains(harian, "workspace.create") {
		t.Error("peristiwa hari ini harus muncul di rentang harian")
	}

	bulanan := getLogs(t, env, uid, "range=month")
	if !strings.Contains(bulanan, "workspace.suspend") {
		t.Error("peristiwa 10 hari lalu HARUS muncul di rentang bulanan — tab-nya menjanjikan itu")
	}
}

// TestTrail_FilterJenisAksi: menyaring per KELUARGA aksi (prefiks), bukan per
// action persis — satu pilihan "workspace" mencakup seluruh workspace.*, jadi
// aksi baru ikut tersaring tanpa menuntut daftar di UI diperbarui.
func TestTrail_FilterJenisAksi(t *testing.T) {
	env, uid := setupTest(t)
	now := time.Now()
	seedAudit(t, env, uid, "auth.login", "session", uid, "", now.Add(-time.Minute))
	seedAudit(t, env, uid, "workspace.create", "workspace", env.tenantID, "", now.Add(-2*time.Minute))
	seedAudit(t, env, uid, "workspace.rename", "workspace", env.tenantID, "", now.Add(-3*time.Minute))

	html := getLogs(t, env, uid, "act=workspace")
	if strings.Contains(html, "auth.login") {
		t.Error("filter workspace tak boleh meloloskan aksi auth")
	}
	for _, want := range []string{"workspace.create", "workspace.rename"} {
		if !strings.Contains(html, want) {
			t.Errorf("filter keluarga harus mencakup %q", want)
		}
	}
}

// TestTrail_FilterPerOrang: pertanyaan paling sering saat menyelidiki adalah
// "apa saja yang dilakukan orang ini".
func TestTrail_FilterPerOrang(t *testing.T) {
	env, uid := setupTest(t)
	lain := env.seedMember(t, "pelakulain@local", "member", 0)
	now := time.Now()
	seedAudit(t, env, uid, "workspace.create", "workspace", env.tenantID, "", now.Add(-time.Minute))
	seedAudit(t, env, lain.ID, "workspace.rename", "workspace", env.tenantID, "", now.Add(-2*time.Minute))

	html := getLogs(t, env, uid, "by="+itoa(lain.ID))
	if !strings.Contains(html, "workspace.rename") {
		t.Error("jejak orang yang difilter harus tampil")
	}
	if strings.Contains(html, "workspace.create") {
		t.Error("jejak orang LAIN tak boleh ikut tampil saat difilter per-orang")
	}
}

// TestTrail_FilterRusakTakMengosongkan: ?act= & ?by= datang dari URL yang bisa
// disunting atau di-bookmark sebelum daftar aksinya berubah. Yang tak bisa
// dipenuhi harus jatuh ke "tanpa filter", bukan ke halaman kosong.
//
// `act=%` diuji khusus: nilainya masuk ke LIKE, jadi wildcard yang lolos akan
// diam-diam mengubah arti filter alih-alih ditolak.
func TestTrail_FilterRusakTakMengosongkan(t *testing.T) {
	env, uid := setupTest(t)
	seedAudit(t, env, uid, "workspace.create", "workspace", env.tenantID, "", time.Now().Add(-time.Minute))

	for _, q := range []string{"act=%", "act=_", "act=TIDAKADA", "by=abc", "by=-1", "after=rusak"} {
		if html := getLogs(t, env, uid, q); !strings.Contains(html, "workspace.create") {
			t.Errorf("query %q mengosongkan halaman — nilai tak sah harus jadi 'tanpa filter'", q)
		}
	}
}

// TestTrail_PeristiwaKe21Terjangkau: LIMIT tanpa jalan keluar = peristiwa lama
// mustahil dilihat, persis penyakit yang diperbaiki di /dev/users.
func TestTrail_PeristiwaKe21Terjangkau(t *testing.T) {
	env, uid := setupTest(t)
	now := time.Now()
	// Jarak antar-peristiwa dalam MENIT, bukan jam, dan rentangnya BULANAN.
	//
	// Versi pertama test ini menaruh peristiwa terlama 5 jam ke belakang pada
	// rentang harian, dan itu lulus atau gagal tergantung JAM BERAPA ia
	// dijalankan: sebelum pukul 05:00 lokal, 5 jam lalu jatuh di hari kemarin
	// dan keluar dari jendela — kegagalan yang menunjuk ke paginasi padahal
	// sebabnya rentang. Test yang bergantung waktu berjalan akan menuduh bagian
	// kode yang salah, tepat saat orang paling percaya padanya.
	seedAudit(t, env, uid, "workspace.suspend", "workspace", env.tenantID,
		`{"reason":"paling-lama"}`, now.Add(-time.Duration(pageSize+1)*time.Minute))
	for i := range pageSize {
		seedAudit(t, env, uid, "auth.login", "session", uid, "",
			now.Add(-time.Duration(i)*time.Minute))
	}

	first := getLogs(t, env, uid, "range=month")
	if strings.Contains(first, "paling-lama") {
		t.Fatal("prasyarat meleset: peristiwa terlama seharusnya belum tampil di halaman pertama")
	}
	next := trailCursorFrom(t, first)
	if next == "" {
		t.Fatal("halaman pertama harus menawarkan jalan ke peristiwa lebih lama")
	}
	// Rentang ikut dibawa: tanpa itu halaman kedua kembali ke rentang harian dan
	// menjawab pertanyaan yang berbeda dari yang ditanyakan.
	if second := getLogs(t, env, uid, "range=month&after="+next); !strings.Contains(second, "paling-lama") {
		t.Error("peristiwa ke-21 tak terjangkau — jejaknya masih terpotong")
	}
}

// TestTrail_PaginasiMempertahankanFilter: berpindah halaman sambil membuang
// filter akan menjawab pertanyaan yang tak ditanyakan siapa pun.
func TestTrail_PaginasiMempertahankanFilter(t *testing.T) {
	env, uid := setupTest(t)
	now := time.Now()
	for i := range pageSize + 1 {
		seedAudit(t, env, uid, "workspace.rename", "workspace", env.tenantID, "",
			now.Add(-time.Duration(i)*time.Minute))
	}

	html := getLogs(t, env, uid, "range=month&act=workspace")
	link := trailNextHref(t, html)
	if link == "" {
		t.Fatal("harus ada tautan halaman berikutnya")
	}
	for _, want := range []string{"act=workspace", "range=month", "after="} {
		if !strings.Contains(link, want) {
			t.Errorf("tautan halaman berikutnya kehilangan %q: %s", want, link)
		}
	}
}

// TestTrail_JejakSelamatSetelahPelakuDihapus: actor_user_id ON DELETE SET NULL
// dan target_id sengaja BUKAN FK — keduanya justru agar bukti tak lenyap
// bersama yang dibuktikan. LEFT JOIN di query harus menghormati itu.
func TestTrail_JejakSelamatSetelahPelakuDihapus(t *testing.T) {
	env, uid := setupTest(t)
	pelaku := env.seedMember(t, "akandihapus@local", "member", 0)
	seedAudit(t, env, pelaku.ID, "workspace.suspend", "workspace", env.tenantID,
		`{"reason":"bukti-harus-selamat"}`, time.Now().Add(-time.Minute))

	if _, err := env.h.Pool.Exec(t.Context(), "DELETE FROM users WHERE id = $1", pelaku.ID); err != nil {
		t.Fatalf("hapus pelaku: %v", err)
	}

	html := getLogs(t, env, uid, "")
	if !strings.Contains(html, "bukti-harus-selamat") {
		t.Error("jejak hilang setelah pelakunya dihapus — bukti tak boleh lenyap bersama yang dibuktikan")
	}
	// Kalimatnya tak boleh menggantung tanpa subjek.
	if !strings.Contains(html, "Seseorang") {
		t.Error("pelaku yang sudah dihapus harus disebut eksplisit, bukan dikosongkan")
	}
}

// TestTrail_TakMenampilkanNamaOrangUntukSasaranWorkspace: bug yang ditemukan
// saat membuka jalur baca ini. h.audit menulis target_type="user" untuk SEMUA
// aksi, padahal tujuh aksi mengirim tenants.id sebagai target_id — meng-JOIN
// users di atasnya memampangkan NAMA ORANG yang id-nya kebetulan sama.
func TestTrail_TakMenampilkanNamaOrangUntukSasaranWorkspace(t *testing.T) {
	env, uid := setupTest(t)
	// User yang id-nya SAMA dengan tenant id — inilah tabrakan yang dulu terjadi.
	umpan := env.seedMember(t, "jangan-muncul@local", "member", 0)
	seedAudit(t, env, uid, "workspace.rename", "workspace", umpan.ID, "", time.Now().Add(-time.Minute))

	html := getLogs(t, env, uid, "")
	if strings.Contains(html, "jangan-muncul@local") {
		t.Error("sasaran workspace menampilkan nama USER — JOIN harus dibatasi target_type")
	}
}

// --- helper ---

// trailNextHref memungut href tautan "Lebih lama" dari HALAMAN, bukan merakit
// ulang di test: yang diuji justru apakah jalannya tersedia bagi yang membuka.
func trailNextHref(t *testing.T, html string) string {
	t.Helper()
	i := strings.Index(html, "/dev/logs?")
	for i >= 0 {
		rest := html[i:]
		end := strings.IndexAny(rest, `"'`)
		if end < 0 {
			return ""
		}
		if href := rest[:end]; strings.Contains(href, "after=") {
			return href
		}
		nxt := strings.Index(html[i+1:], "/dev/logs?")
		if nxt < 0 {
			return ""
		}
		i += 1 + nxt
	}
	return ""
}

func trailCursorFrom(t *testing.T, html string) string {
	t.Helper()
	href := trailNextHref(t, html)
	if href == "" {
		return ""
	}
	_, after, found := strings.Cut(href, "after=")
	if !found {
		return ""
	}
	if amp := strings.Index(after, "&"); amp >= 0 {
		after = after[:amp]
	}
	return after
}

// firstLines memotong HTML untuk pesan error yang terbaca.
func firstLines(s string) string {
	if len(s) > 600 {
		return s[:600] + "…"
	}
	return s
}
