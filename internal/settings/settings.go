// Package settings menyimpan pengaturan platform yang bisa diubah saat jalan
// (tanpa restart), dengan cache in-process agar tak menambah query di jalur
// panas.
//
// Kenapa ada: kuota workspace dulu hanya di env MAX_WORKSPACES_PER_USER, dan env
// itu cuma dipakai saat user DIBUAT lalu disalin ke users.workspace_quota.
// Mengubahnya tak berpengaruh sama sekali pada user yang sudah ada — itu "nilai
// awal", bukan "aturan global". Di sini nilainya hidup di DB, dibaca lewat
// cache, dan berubah SEKETIKA bagi semua yang belum di-override.
package settings

import (
	"strconv"
	"sync"
)

// Kunci pengaturan. Konstanta, bukan string bebas: typo pada key hanya akan
// terlihat sebagai "nilainya kembali ke default" — kegagalan senyap yang mahal
// dilacak (pola yang sama dengan key session di internal/session).
const (
	KeyWorkspaceQuotaDefault = "workspace_quota_default"
)

// Batas kuota yang masuk akal. Bukan cuma jaga-jaga input: 0 akan mengunci
// SEMUA orang dari membuat workspace (termasuk pendaftar baru yang belum punya
// satu pun), dan angka raksasa menjadikan kuota tak berarti sambil membuka jalan
// penyalahgunaan sumber daya.
const (
	MinWorkspaceQuota = 1
	MaxWorkspaceQuota = 100
)

// store = cache in-process. Pengaturan sangat jarang berubah tapi dibaca di tiap
// pembuatan workspace & render sidebar; membacanya dari DB tiap kali menambah
// query tanpa manfaat.
//
// Ini cache PER-PROSES: pada deployment multi-instance, perubahan lewat panel
// hanya seketika di instance yang melayaninya — yang lain menyusul saat boot
// berikutnya. Dapat diterima karena satu-satunya penulis adalah operator
// platform, dan efek terburuknya sementara: user diizinkan/ditolak membuat satu
// workspace di luar aturan terbaru selama beberapa saat. Kalau kelak butuh
// serempak, jalurnya pub/sub Redis (sudah ada di stack) — bukan menghapus cache.
var store = struct {
	sync.RWMutex
	values   map[string]string
	fallback map[string]string
}{
	values:   map[string]string{},
	fallback: map[string]string{},
}

// SetFallback menetapkan nilai bawaan dari config/env — dipakai saat baris di DB
// belum ada (deployment baru). Dipanggil sekali saat startup, SEBELUM Load.
func SetFallback(key, value string) {
	store.Lock()
	defer store.Unlock()
	store.fallback[key] = value
}

// Load mengisi cache dari daftar pengaturan yang dibaca pemanggil dari DB.
// Menerima map (bukan mengakses DB sendiri) agar paket ini tak bergantung pada
// db/pgx — sekaligus membuatnya bisa diuji tanpa database.
func Load(kv map[string]string) {
	store.Lock()
	defer store.Unlock()
	store.values = kv
}

// Set memperbarui satu nilai di cache (dipanggil setelah tulis ke DB sukses).
// Urutannya penting: DB dulu, cache kemudian — kalau terbalik, tulis yang gagal
// meninggalkan cache berbohong sampai proses restart.
func Set(key, value string) {
	store.Lock()
	defer store.Unlock()
	store.values[key] = value
}

// All mengembalikan salinan seluruh nilai efektif (DB ditimpa di atas fallback).
// Salinan, bukan map aslinya: pemanggil tak boleh bisa mengubah cache diam-diam.
func All() map[string]string {
	store.RLock()
	defer store.RUnlock()
	out := make(map[string]string, len(store.fallback)+len(store.values))
	for k, v := range store.fallback {
		out[k] = v
	}
	for k, v := range store.values {
		out[k] = v
	}
	return out
}

// Int membaca pengaturan bertipe angka. Urutan: nilai DB → fallback env → def.
// Nilai tak terparse diperlakukan seperti tak ada (jatuh ke fallback): satu
// baris rusak di DB tak boleh membuat pembuatan workspace gagal total.
func Int(key string, def int) int {
	store.RLock()
	v, ok := store.values[key]
	fb, hasFB := store.fallback[key]
	store.RUnlock()

	if ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	if hasFB {
		if n, err := strconv.Atoi(fb); err == nil {
			return n
		}
	}
	return def
}

// WorkspaceQuotaDefault = jatah workspace untuk user yang BELUM di-override
// (users.workspace_quota IS NULL). Berubah seketika saat operator mengubahnya.
func WorkspaceQuotaDefault() int {
	return Int(KeyWorkspaceQuotaDefault, MinWorkspaceQuota)
}

// EffectiveWorkspaceQuota menghitung jatah yang BERLAKU untuk satu user:
// override per-user bila ada, selainnya default global.
//
// Satu-satunya tempat aturan ini dirumuskan — handler tak boleh menghitungnya
// sendiri, karena "mana yang menang" adalah keputusan yang harus sama persis di
// titik penegakan (boleh/tidak membuat) dan di titik tampilan (sidebar).
// Berbeda sedikit saja = user melihat sisa jatah yang tak sesuai kenyataan.
func EffectiveWorkspaceQuota(userOverride *int32) int {
	if userOverride != nil {
		return int(*userOverride)
	}
	return WorkspaceQuotaDefault()
}

// ValidWorkspaceQuota melaporkan apakah angka masuk akal sebagai kuota.
// Divalidasi di backend, bukan hanya di form: nilainya datang dari input
// user-controlled (Rule Datastar #15b).
func ValidWorkspaceQuota(n int) bool {
	return n >= MinWorkspaceQuota && n <= MaxWorkspaceQuota
}
