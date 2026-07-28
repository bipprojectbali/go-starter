package settings

import "testing"

// settings_test.go — aturan kuota. Yang dijaga bukan "fungsi mengembalikan
// angka", melainkan pembedaan yang jadi alasan seluruh perubahan ini ada:
// NULL (ikut global, ikut berubah) vs angka (hak khusus, kebal perubahan).

// reset mengembalikan store ke keadaan bersih antar-test (state paket).
func reset(t *testing.T) {
	t.Helper()
	store.Lock()
	store.values = map[string]string{}
	store.fallback = map[string]string{}
	store.Unlock()
}

// TestEffectiveQuota_NilIkutGlobal: user tanpa override mengikuti default —
// DAN ikut berubah saat default diubah. Inilah yang mustahil dilakukan kolom
// INT NOT NULL, tempat setiap user memegang salinan angkanya sendiri.
func TestEffectiveQuota_NilIkutGlobal(t *testing.T) {
	reset(t)
	Load(map[string]string{KeyWorkspaceQuotaDefault: "3"})

	if got := EffectiveWorkspaceQuota(nil); got != 3 {
		t.Fatalf("tanpa override harus ikut global 3, got %d", got)
	}
	// Operator menaikkan default → user yang sama langsung ikut, tanpa disentuh.
	Set(KeyWorkspaceQuotaDefault, "7")
	if got := EffectiveWorkspaceQuota(nil); got != 7 {
		t.Errorf("perubahan global harus SEKETIKA berlaku, got %d", got)
	}
}

// TestEffectiveQuota_OverrideMenang: hak khusus kebal perubahan global — itu
// gunanya diberikan. Kalau ikut berubah, ia bukan hak khusus.
func TestEffectiveQuota_OverrideMenang(t *testing.T) {
	reset(t)
	Load(map[string]string{KeyWorkspaceQuotaDefault: "1"})

	khusus := int32(5)
	if got := EffectiveWorkspaceQuota(&khusus); got != 5 {
		t.Fatalf("override harus menang atas global, got %d", got)
	}
	Set(KeyWorkspaceQuotaDefault, "9")
	if got := EffectiveWorkspaceQuota(&khusus); got != 5 {
		t.Errorf("hak khusus TAK boleh ikut berubah saat global naik, got %d", got)
	}
	// Termasuk saat hak khusus LEBIH KECIL dari global — pembatasan perorangan
	// sama sahnya dengan kelonggaran perorangan.
	kecil := int32(1)
	if got := EffectiveWorkspaceQuota(&kecil); got != 1 {
		t.Errorf("override lebih kecil dari global harus tetap menang, got %d", got)
	}
}

// TestInt_FallbackEnv: baris DB belum ada (deployment baru) → pakai nilai env.
// Tanpa ini, instalasi baru mulai dengan kuota bawaan kode dan mengabaikan
// MAX_WORKSPACES_PER_USER yang sudah diset operator.
func TestInt_FallbackEnv(t *testing.T) {
	reset(t)
	SetFallback(KeyWorkspaceQuotaDefault, "4")

	if got := WorkspaceQuotaDefault(); got != 4 {
		t.Fatalf("tanpa baris DB harus jatuh ke fallback env, got %d", got)
	}
	// Begitu DB punya nilainya, DB yang menang — itu yang bisa diubah operator.
	Load(map[string]string{KeyWorkspaceQuotaDefault: "8"})
	if got := WorkspaceQuotaDefault(); got != 8 {
		t.Errorf("nilai DB harus menang atas fallback env, got %d", got)
	}
}

// TestInt_NilaiRusakTakMematikan: satu baris rusak di DB tak boleh membuat
// pembuatan workspace gagal total — jatuh ke fallback, bukan ke 0 (yang akan
// mengunci SEMUA orang).
func TestInt_NilaiRusakTakMematikan(t *testing.T) {
	reset(t)
	SetFallback(KeyWorkspaceQuotaDefault, "3")
	Load(map[string]string{KeyWorkspaceQuotaDefault: "bukan-angka"})

	if got := WorkspaceQuotaDefault(); got != 3 {
		t.Errorf("nilai rusak harus jatuh ke fallback, got %d", got)
	}
}

// TestValidWorkspaceQuota: 0 akan mengunci semua orang dari membuat workspace
// (termasuk pendaftar baru yang belum punya satu pun) — bukan sekadar input
// aneh, tapi penguncian diri seluruh platform.
func TestValidWorkspaceQuota(t *testing.T) {
	cases := map[int]bool{
		0:                     false, // mengunci semua orang
		-1:                    false,
		MinWorkspaceQuota:     true,
		5:                     true,
		MaxWorkspaceQuota:     true,
		MaxWorkspaceQuota + 1: false, // kuota jadi tak berarti
	}
	for n, want := range cases {
		if got := ValidWorkspaceQuota(n); got != want {
			t.Errorf("ValidWorkspaceQuota(%d) = %v, want %v", n, got, want)
		}
	}
}

// TestAll_SalinanBukanReferensi: pemanggil tak boleh bisa mengubah cache diam-
// diam lewat map yang dikembalikan.
func TestAll_SalinanBukanReferensi(t *testing.T) {
	reset(t)
	Load(map[string]string{KeyWorkspaceQuotaDefault: "3"})

	got := All()
	got[KeyWorkspaceQuotaDefault] = "999"

	if WorkspaceQuotaDefault() != 3 {
		t.Error("mengubah hasil All() tak boleh menyentuh cache")
	}
}
