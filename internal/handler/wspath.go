package handler

import (
	"context"
	"net/http"
	"strings"

	"go_starter/internal/appmode"
	"go_starter/internal/authz"
	"go_starter/internal/session"

	"github.com/go-chi/chi/v5"
)

// wspath.go — pembentukan URL workspace. SATU-SATUNYA tempat literal "/w/"
// muncul (Rule 15): setelah 0004 setiap tautan workspace bergantung slug, jadi
// path tak boleh lagi ditulis tangan yang tersebar.

// WorkspacePrefix = segmen akar route ber-workspace di mode MULTI. Prefix
// eksplisit (bukan "/{slug}" telanjang) membuat tabrakan slug-vs-route MUSTAHIL
// secara struktural — tak perlu daftar kata terlarang yang dijaga selamanya.
const WorkspacePrefix = "/w"

// SingleAppPrefix = akar aplikasi di mode SINGLE. Tanpa segmen slug: user tak
// pernah melihat kata "workspace", padahal di dalam tetap ada tepat satu tenant
// (0006 §1 — bukan jalur kode kedua).
const SingleAppPrefix = "/app"

// slugURLParam = nama parameter chi di pola route ("/w/{workspace}/...").
// Di mode single tak ada parameter ini — chi.URLParam mengembalikan "".
const slugURLParam = "workspace"

// wsPath membentuk URL halaman di dalam ruang kerja/aplikasi:
//
//	multi  → wsPath("acme", "/members") = "/w/acme/members"
//	single → wsPath(_,      "/members") = "/app/members"
//
// SATU-SATUNYA tempat bentuk itu diputuskan (0006 §3): karena seluruh handler &
// view sudah lewat sini sejak 0004, mengubah bentuk URL antar-mode tak menyentuh
// satu pun pemanggilnya.
//
// Slug kosong di mode MULTI (belum punya workspace) → "/workspace/new":
// satu-satunya tujuan masuk akal, sekaligus mencegah URL rusak "/w//members".
// Di mode single slug memang selalu diabaikan — tak ada keadaan "belum punya".
func wsPath(slug, sub string) string {
	base := SingleAppPrefix
	if !appmode.IsSingle() {
		if slug == "" {
			return "/workspace/new"
		}
		base = WorkspacePrefix + "/" + slug
	}
	if sub == "" || sub == "/" {
		return base
	}
	if !strings.HasPrefix(sub, "/") {
		sub = "/" + sub
	}
	return base + sub
}

// wsPathOf = wsPath memakai workspace aktif di session. Dipakai jalur yang tak
// punya slug di URL-nya sendiri (redirect setelah login, landing, /notifications).
func wsPathOf(ctx context.Context, sub string) string {
	return wsPath(session.TenantSlug(ctx), sub)
}

// slugFromRequest membaca slug workspace dari path route ("" bila route ini tak
// ber-workspace, mis. /dev atau /notifications).
//
// Di mode SINGLE selalu mengembalikan slug tenant tunggal, karena route /app tak
// punya segmen slug untuk dibaca. Ini yang membuat 0006 murah: middleware Scope,
// resolveTenantBySlug, wsRedirect, dan gerbang siklus hidup semuanya bekerja
// APA ADANYA — mereka menerima slug, dan di mode single slug itu selalu sama.
// Tanpa ini, tiap pemanggil harus tahu sedang di mode apa.
func slugFromRequest(r *http.Request) string {
	if appmode.IsSingle() {
		return appmode.SingleSlug
	}
	return chi.URLParam(r, slugURLParam)
}

// wsRedirect mengalihkan (303) ke halaman di dalam workspace request ini,
// opsional dengan kode error PRG (?err=CODE). Slug diambil dari path — bukan
// session — agar redirect selalu kembali ke workspace yang SEDANG dibuka, bukan
// ke yang kebetulan aktif di cookie.
//
// Ada karena tiap handler workspace punya banyak cabang gagal; tanpa ini, literal
// path berulang di puluhan tempat (Rule 15) dan satu saja yang lupa diperbarui
// akan melempar user ke workspace lain secara senyap.
func wsRedirect(w http.ResponseWriter, r *http.Request, sub, errCode string) {
	url := wsPath(slugFromRequest(r), sub)
	if errCode != "" {
		url += "?err=" + errCode
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// homeFor mengembalikan tujuan "rumah" setelah login/dari landing. Role PLATFORM
// → /dev (lintas-workspace, tak punya slug); role tenant → akar workspace aktif.
//
// Menggantikan authz.HomePath untuk role tenant: sejak 0004 alamatnya bergantung
// WORKSPACE, bukan role. Orang yang sama owner di A & member di B — dulu itu
// berarti /admin vs /user, yang membuat alamat halaman yang sama berubah saat
// pindah workspace.
func homeFor(ctx context.Context) string {
	if role := session.Role(ctx); isPlatformRole(role) || session.IsRoot(ctx) {
		return authz.PlatformHomePath
	}
	return wsPathOf(ctx, "")
}
