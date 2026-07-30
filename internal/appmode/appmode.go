// Package appmode menyimpan bentuk aplikasi: satu app (single) atau
// multi-tenant. Dipisah jadi paket sendiri — bukan konstanta di handler — supaya
// bisa dibaca config, main, handler, dan view tanpa saling impor.
//
// PENTING (keputusan 0006): mode single BUKAN jalur kode kedua. Di dalam, ia
// tetap multi-tenant dengan TEPAT SATU tenant — RLS, memberships, audit berjalan
// apa adanya. Yang berbeda hanya chrome yang ditampilkan; bentuk URL pun sama
// (/w/{slug}), dengan workspace primer ber-slug tetap "app".
//
// SUMBER MODE = DATABASE, bukan env (keputusan 0007). Env bisa dibalik: menurunkan
// APP_MODE kembali ke `single` setelah ada banyak workspace menyembunyikan sisanya
// — kehilangan data yang tampak seperti bug UI. Sekarang nilainya hidup di
// platform_settings dengan trigger yang MENOLAK penurunan, jadi mustahil secara
// konstruksi alih-alih dicegah oleh kode yang ingat memeriksa.
//
// Nol baris = single. Deployment baru jadi otomatis satu-aplikasi tanpa siapa pun
// perlu mengisi apa pun.
package appmode

// Mode = bentuk aplikasi. Tipe bertipe (bukan bool/string bebas): nilai ngawur
// jadi kesalahan yang terlihat, bukan diam-diam jatuh ke salah satu mode.
type Mode int

const (
	// Single = satu aplikasi — DEFAULT, dan itu disengaja (keputusan 0007).
	// Setiap aplikasi lahir sebagai satu aplikasi; multi-tenant adalah sesuatu
	// yang DINAIKKAN, bukan yang harus dimatikan. Default yang bisa ditingkatkan
	// jauh lebih aman daripada default yang harus diturunkan: penurunan
	// menyembunyikan data, kenaikan tidak.
	Single Mode = iota
	// Multi = multi-tenant, banyak workspace per user. Sekali di sini, tak bisa
	// kembali — trigger DB menolaknya.
	Multi
)

// Nilai yang disimpan di platform_settings. Konstanta agar typo di satu tempat
// tak diam-diam berarti mode lain.
const (
	NameMulti  = "multi"
	NameSingle = "single"
)

// SettingKey = kunci baris platform_settings tempat mode disimpan. Nilainya
// dijaga trigger DB (multi tak bisa turun, barisnya tak bisa dihapus).
const SettingKey = "tenancy_mode"

// PrimarySlug = slug workspace PRIMER — rumah aplikasi itu sendiri.
//
// KONSTAN, dan tetap dipakai setelah naik ke multi. Inilah yang membuat kenaikan
// mode tak mematikan satu tautan pun: /w/app hari ini adalah /w/app besok, cuma
// kini ada tetangganya. Dulu mode single memakai bentuk URL sendiri (/app/...),
// sehingga kenaikan mode berarti setiap alamat yang sudah tersebar berubah.
const PrimarySlug = "app"

// Parse memetakan nilai tersimpan ke Mode. Kosong / tak dikenal → Single: baris
// yang belum ada berarti aplikasi belum pernah dinaikkan. Bool kedua = false bila
// nilainya TERISI tapi tak dikenal — itu data rusak, bukan keadaan awal, dan
// pemanggil memakainya untuk gagal-cepat alih-alih diam-diam menjalankan mode
// yang tak diminta siapa pun.
func Parse(s string) (Mode, bool) {
	switch s {
	case "", NameSingle:
		return Single, true
	case NameMulti:
		return Multi, true
	default:
		return Single, false
	}
}

// String mengembalikan nama mode (untuk log & pesan galat).
func (m Mode) String() string {
	if m == Multi {
		return NameMulti
	}
	return NameSingle
}

// current = mode aktif, di-set saat startup dari DB (dan saat operator
// menaikkannya). Bukan diteruskan lewat parameter karena dibutuhkan di titik yang
// tak punya jalur argumen: view helper dan pendaftaran route.
var current = Single

// Set menetapkan mode aplikasi: saat startup (dibaca dari DB) dan saat operator
// menaikkannya ke multi.
//
// Aman dipanggil saat melayani: kenaikan hanya MENAMBAH yang boleh dilihat
// (menu ganti-workspace, tombol buat-workspace). Route tak perlu didaftarkan
// ulang — seluruhnya sudah berbentuk /w/{slug} sejak mode single.
//
// Arah sebaliknya tak dijaga di sini melainkan di DATABASE (trigger menolak
// penurunan): penjagaan yang bisa dilewati dengan memanggil fungsi lain bukanlah
// penjagaan.
func Set(m Mode) { current = m }

// Current mengembalikan mode aktif.
func Current() Mode { return current }

// IsSingle melaporkan apakah aplikasi berjalan sebagai satu app.
func IsSingle() bool { return current == Single }

// IsMulti melaporkan apakah aplikasi berjalan multi-tenant.
func IsMulti() bool { return current == Multi }
