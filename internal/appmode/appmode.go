// Package appmode menyimpan bentuk aplikasi: satu app (single) atau
// multi-tenant. Dipisah jadi paket sendiri — bukan konstanta di handler — supaya
// bisa dibaca config, main, handler, dan view tanpa saling impor.
//
// PENTING (keputusan 0006): mode single BUKAN jalur kode kedua. Di dalam, ia
// tetap multi-tenant dengan TEPAT SATU tenant — RLS, memberships, audit berjalan
// apa adanya. Yang berbeda hanya bentuk URL dan chrome yang ditampilkan.
// Membuang tenant_id "karena toh cuma satu" akan melahirkan aplikasi kedua di
// dalam satu repo, dan jalur yang lebih jarang dipakai pasti membusuk.
package appmode

// Mode = bentuk aplikasi. Tipe bertipe (bukan bool/string bebas): nilai ngawur
// jadi kesalahan yang terlihat, bukan diam-diam jatuh ke salah satu mode.
type Mode int

const (
	// Multi = multi-tenant, URL /w/{slug}/… — DEFAULT, agar turunan yang tak
	// mengisi APP_MODE berperilaku persis seperti template hari ini.
	Multi Mode = iota
	// Single = satu aplikasi, URL /app/… User tak pernah melihat kata "workspace".
	Single
)

// Nilai env yang diterima. Konstanta agar typo di satu tempat tak diam-diam
// berarti mode lain.
const (
	NameMulti  = "multi"
	NameSingle = "single"
)

// SingleSlug = slug tenant tunggal di mode single. KONSTAN (bukan dari APP_NAME)
// supaya path selalu "/app" di semua deployment — dokumentasi turunan bisa
// menyebut alamat yang pasti, dan mengganti nama aplikasi tak mematikan tautan.
const SingleSlug = "app"

// Parse memetakan nilai env ke Mode. Nilai kosong/tak dikenal → Multi (default
// aman: perilaku template tak berubah). Bool kedua = false bila nilainya diisi
// TAPI tak dikenal — pemanggil (config) memakainya untuk gagal-cepat alih-alih
// diam-diam menjalankan mode yang tak diminta siapa pun.
func Parse(s string) (Mode, bool) {
	switch s {
	case "", NameMulti:
		return Multi, true
	case NameSingle:
		return Single, true
	default:
		return Multi, false
	}
}

// String mengembalikan nama mode (untuk log & pesan galat).
func (m Mode) String() string {
	if m == Single {
		return NameSingle
	}
	return NameMulti
}

// current = mode aktif, di-set sekali saat startup (pola SetDevMode/SetCSSPath).
// Bukan diteruskan lewat parameter karena dibutuhkan di titik-titik yang tak
// punya jalur argumen: pembentukan URL di view helper dan pendaftaran route.
var current = Multi

// Set menetapkan mode aplikasi. Dipanggil SEKALI saat startup, sebelum route
// didaftarkan — mode menentukan bentuk route, dan route dipasang sekali.
func Set(m Mode) { current = m }

// Current mengembalikan mode aktif.
func Current() Mode { return current }

// IsSingle melaporkan apakah aplikasi berjalan sebagai satu app.
func IsSingle() bool { return current == Single }

// IsMulti melaporkan apakah aplikasi berjalan multi-tenant.
func IsMulti() bool { return current == Multi }
