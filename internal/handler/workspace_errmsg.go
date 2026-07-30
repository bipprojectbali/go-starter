package handler

// workspace_errmsg.go — peta kode PRG (`?err=CODE`) → pesan untuk halaman
// workspace & platform. SATU tempat, sengaja.
//
// Kenapa dipusatkan: peta seperti ini sempat tersebar, dan yang tersebar mudah
// jadi setengah lengkap — handler mengirim kode baru, petanya lupa diperbarui,
// lalu penolakannya HILANG SENYAP: user menekan tombol, halaman termuat ulang
// tampak normal, tak ada penjelasan apa pun.
//
// Itu bukan kekhawatiran teoretis. Dua kejadian nyata saat menulis ini:
//   - `?err=primary` (tolak arsip/hapus rumah aplikasi) tak pernah dirender
//     karena halaman pengaturan tak punya slot pesan sama sekali;
//   - `members.go` sudah lama mengirim `?err=forbidden` sementara petanya tak
//     mengenal kode itu — jadi setiap penolakan di halaman anggota tampil
//     sebagai halaman yang sekadar termuat ulang.
//
// Kode ringkas di URL (bukan kalimat penuh) agar rapi dan tak membocorkan detail
// internal. Kode tak dikenal → "" (tak ada alert): diam lebih baik daripada
// "error tak dikenal", yang membingungkan tanpa memberi informasi.

// wsErrMsg memetakan kode untuk seluruh halaman ber-workspace (pengaturan,
// anggota, daftar workspace platform).
//
// Satu peta, bukan satu per halaman: kode yang sama harus berbunyi sama di mana
// pun ia muncul, dan `failed`/`forbidden` memang dipakai hampir di semua jalur.
func wsErrMsg(code string) string {
	switch code {
	case "primary":
		return "Workspace ini adalah rumah aplikasi — tak bisa diarsipkan maupun dihapus."
	case "forbidden":
		return "Anda tidak berwenang melakukan tindakan ini."
	case "reason":
		return "Alasan wajib diisi."
	case "name":
		return "Nama workspace wajib diisi (maks 60 karakter)."
	case "quota":
		return "Kuota workspace Anda sudah penuh."
	case "failed":
		return "Tindakan gagal. Coba lagi."
	default:
		return ""
	}
}
