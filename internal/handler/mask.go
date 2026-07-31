package handler

import "strings"

// mask.go — penyamaran PII untuk mata yang tak berhak melihatnya.
//
// Kenapa di HANDLER, bukan di view: kalau view yang menyamarkan, email ASLI tetap
// harus dioper ke sana — dan satu pemakaian yang lupa menyamarkan akan
// mengirimnya ke browser, tempat ia terbaca di source meski tak tampak di layar.
// Dengan menyamarkan di sini, yang tak berhak melihatnya TAK PERNAH menerimanya.
// Ini konsisten dengan aturan "view murni-data": view menerima yang siap dirender.

// maskEmail menyamarkan alamat email jadi bentuk yang masih bisa DIKENALI tapi
// tak bisa dipakai menghubungi orangnya: "malikkurosaki@gmail.com" →
// "mal•••@gmail.com".
//
// Kenapa disamarkan, bukan dihapus: ia CADANGAN penanda orang. Sejak `users`
// punya kolom `name` (migrasi 00002), penanda utama di layar adalah nama — dan
// email tak lagi perlu tampil sama sekali bagi yang tak mengelola. Tapi nama
// boleh kosong (akun password dev, provider yang tak mengirimkannya), dan tanpa
// cadangan ini baris tanpa nama jadi avatar anonim yang tak bisa dibedakan dari
// baris tanpa nama lainnya — daftar anggota kehilangan gunanya justru untuk
// orang yang paling sulit dikenali.
//
// Catatan sejarah yang menjelaskan bentuknya: fungsi ini dulu KOMPENSASI atas
// ketiadaan kolom nama, jadi ia dirancang agar bentuk samarannya masih bisa
// dikenali manusia. Sekarang perannya menyusut, tapi syarat itu tetap berlaku.
//
// Domain DIPERTAHANKAN dengan sengaja: itulah yang membedakan rekan satu
// organisasi dari orang luar, dan justru pertanyaan itu yang membuat daftar
// anggota berguna bagi anggota biasa.
//
// Panjang bagian lokal TIDAK dibocorkan (selalu tiga titik, bukan sejumlah
// karakter aslinya) — panjang mempersempit ruang tebak.
func maskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		// Bukan bentuk email (atau bagian lokalnya kosong). Jangan pura-pura bisa
		// menyamarkan sesuatu yang tak dikenali strukturnya — sembunyikan penuh.
		return "•••"
	}
	local, domain := email[:at], email[at:]

	// Bagian lokal pendek: memperlihatkan 3 dari 4 karakter praktis sama dengan
	// tak menyamarkan apa pun, jadi yang ditampilkan menyusut bersamanya.
	keep := 3
	switch {
	case len(local) <= 2:
		keep = 1
	case len(local) <= 4:
		keep = 2
	}
	return local[:keep] + "•••" + domain
}
