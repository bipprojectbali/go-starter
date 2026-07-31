package preflight

import (
	"fmt"
	"strings"
)

// explain.go — mengubah kegagalan jadi petunjuk.
//
// Pemisahan ini disengaja: preflight.go menjawab "apa yang salah", file ini
// menjawab "lalu saya harus apa". Yang kedua tumbuh mengikuti kekeliruan yang
// benar-benar dilakukan orang, bukan mengikuti bentuk pemeriksaannya.

// Problem = satu hal yang kurang, beserta cara memperbaikinya.
type Problem struct {
	What string   // apa yang salah, satu kalimat
	Why  string   // kenapa itu penting; "" bila sudah jelas dari What
	Fix  []string // perintah/langkah persis — bukan "periksa konfigurasi Anda"
}

// Report = kumpulan masalah yang ditemukan satu kali jalan.
//
// SEMUA masalah dikumpulkan, bukan berhenti di yang pertama: memperbaiki satu
// lalu menemukan yang berikutnya membuat orang menjalankan perintah yang sama
// empat kali. Yang ingin diketahui adalah "apa saja yang kurang", sekaligus.
type Report struct {
	Problems []Problem
}

// Add mencatat satu atau beberapa masalah. Variadic supaya pemeriksaan yang
// tak menemukan apa-apa bisa mengembalikan slice kosong tanpa pemanggil perlu
// memeriksanya lebih dulu.
func (r *Report) Add(ps ...Problem) { r.Problems = append(r.Problems, ps...) }

// OK melaporkan apakah lingkungannya siap.
func (r *Report) OK() bool { return len(r.Problems) == 0 }

// String merender laporan sebagai teks siap-tempel di terminal.
func (r *Report) String() string {
	if r.OK() {
		return "Lingkungan siap."
	}
	var b strings.Builder
	n := len(r.Problems)
	fmt.Fprintf(&b, "\n%d hal perlu dibereskan sebelum aplikasi bisa jalan:\n", n)
	for i, p := range r.Problems {
		fmt.Fprintf(&b, "\n  %d. %s\n", i+1, p.What)
		if p.Why != "" {
			fmt.Fprintf(&b, "     %s\n", p.Why)
		}
		for _, f := range p.Fix {
			fmt.Fprintf(&b, "     $ %s\n", f)
		}
	}
	return b.String()
}

// DatabaseMissing menyusun petunjuk untuk database yang belum dibuat.
//
// similar = nama database yang ADA dan mirip. Ini bagian yang paling sering
// menyelesaikan masalah dalam sekali baca: yang keliru hampir selalu beda tipis
// (tanda hubung vs garis bawah, `_test` tertinggal, nama sebelum rename), dan
// menunjukkannya membuat orang mengenali salah ketiknya sendiri.
func DatabaseMissing(t DBTarget, similar []string) Problem {
	p := Problem{
		What: fmt.Sprintf("Database %q belum ada di %s:%s.", t.DBName, t.Host, t.Port),
		Fix:  []string{"createdb " + t.DBName},
	}
	if len(similar) > 0 {
		p.Why = fmt.Sprintf("Tapi ada yang mirip: %s — periksa DATABASE_URL di .env, "+
			"mungkin namanya tertukar.", strings.Join(quoteAll(similar), ", "))
	}
	return p
}

// PostgresDown menyusun petunjuk untuk server yang tak bisa dihubungi.
//
// Dibedakan dari database-yang-belum-ada, sebab tindakannya berbeda sama
// sekali: yang satu membuat database, yang lain menyalakan server. Menyatukan
// keduanya jadi "gagal konek" memaksa orang mendiagnosis sendiri hal yang sudah
// diketahui program.
func PostgresDown(t DBTarget, err error) Problem {
	return Problem{
		What: fmt.Sprintf("Postgres di %s:%s tak bisa dihubungi.", t.Host, t.Port),
		Why:  "Pesan asli: " + firstLine(err.Error()),
		Fix: []string{
			"brew services start postgresql@17   # macOS",
			"sudo systemctl start postgresql     # Linux",
		},
	}
}

// RedisDown menyusun petunjuk untuk Redis yang tak bisa dihubungi.
func RedisDown(addr string, err error) Problem {
	return Problem{
		What: fmt.Sprintf("Redis di %s tak bisa dihubungi.", addr),
		Why:  "Session disimpan di Redis, jadi tanpa ini tak ada yang bisa login. Pesan asli: " + firstLine(err.Error()),
		Fix: []string{
			"brew services start redis   # macOS",
			"sudo systemctl start redis  # Linux",
		},
	}
}

// EnvMissing menyusun petunjuk untuk .env yang belum disalin.
func EnvMissing() Problem {
	return Problem{
		What: "File .env belum ada.",
		Why:  "Aplikasi membaca DATABASE_URL, REDIS_ADDR, dan SESSION_KEY dari sana.",
		Fix:  []string{"cp .env.example .env"},
	}
}

// NameMismatch menyusun peringatan saat nama di .env tak cocok dengan nama
// module — gejala `make rename` yang belum diikuti penyesuaian .env.
//
// Ini yang PALING sulit didiagnosis sendiri: keduanya "benar" bila dilihat
// terpisah, dan yang keliru cuma bahwa keduanya tak menunjuk hal yang sama.
func NameMismatch(module, dbName string) Problem {
	return Problem{
		What: fmt.Sprintf("Nama module (%q) dan nama database di .env (%q) tak cocok.", module, dbName),
		Why: "Biasanya karena `make rename` sudah dijalankan tapi .env belum disesuaikan " +
			"(.env sengaja tak ikut diubah — isinya kredensial nyata).",
		Fix: []string{
			fmt.Sprintf("# sunting .env: DATABASE_URL → .../%s", module),
			fmt.Sprintf("# lalu: createdb %s", module),
		},
	}
}

func quoteAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}

// firstLine memotong pesan error ke baris pertama. Error jaringan Go kerap
// panjang & berulang; yang menjelaskan sebabnya selalu di depan.
func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
