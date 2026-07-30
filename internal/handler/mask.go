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
// Kenapa disamarkan, bukan dihapus: tabel `users` tak punya kolom nama, jadi
// membuang email sama dengan membuang satu-satunya penanda yang membedakan satu
// baris dari yang lain — daftar anggota jadi kumpulan avatar anonim, dan
// gunanya (tahu SIAPA yang punya akses ke ruang ini) hilang bersamanya.
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
