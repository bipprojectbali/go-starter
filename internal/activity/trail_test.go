package activity

import (
	"strings"
	"testing"
)

// trail_test.go — kalimat peristiwa audit.
//
// Yang dijaga: (1) tiap action yang KITA tulis punya kalimatnya sendiri, (2)
// action asing tak menghilangkan barisnya, (3) nama yang hilang tak membuat
// kalimatnya menggantung.

// knownActions = semua action yang ditulis handler. Daftar ini sengaja
// dieja ulang di test, bukan diimpor dari kode: kalau ia diturunkan dari sumber
// yang sama dengan yang diuji, keduanya bisa salah bersamaan dan test-nya tetap
// hijau. Menambah action baru tanpa kalimatnya akan gagal di sini.
var knownActions = []string{
	"auth.login", "auth.logout",
	"member.role.update", "member.remove",
	"invite.create", "invite.accept",
	"workspace.create", "workspace.rename", "workspace.archive",
	"workspace.unarchive", "workspace.delete", "workspace.suspend",
	"user.status.update", "user.delete", "user.quota.set", "user.quota.reset",
	"settings.workspace_quota", "platform.tenancy.upgrade",
}

// TestSentence_SemuaActionPunyaKalimat: tak boleh ada yang jatuh ke cabang
// default. Yang jatuh ke sana terbaca sebagai "melakukan tindakan
// workspace.create" — benar secara harfiah, tapi itu justru bentuk kegagalan
// yang tak terlihat seperti kegagalan.
func TestSentence_SemuaActionPunyaKalimat(t *testing.T) {
	for _, act := range knownActions {
		got := Sentence(Event{Action: act, Actor: "Budi", TargetUser: "Siti", Workspace: "Acme"})
		if strings.Contains(got, "melakukan tindakan") {
			t.Errorf("action %q jatuh ke kalimat default — kalimatnya belum ditulis", act)
		}
		if !strings.HasSuffix(got, ".") {
			t.Errorf("action %q: kalimat harus diakhiri titik, got %q", act, got)
		}
		if strings.Contains(got, act) {
			t.Errorf("action %q: kode mentah bocor ke kalimat: %q", act, got)
		}
	}
}

// TestSentence_ActionAsingTetapTerbaca: baris lama dari versi lain, atau action
// yang belum dikenal. Jejak audit dibaca justru saat ada yang aneh — baris yang
// menghilang karena kodenya tak dikenal adalah kegagalan terburuk halaman ini.
func TestSentence_ActionAsingTetapTerbaca(t *testing.T) {
	got := Sentence(Event{Action: "sesuatu.yang.baru", Actor: "Budi"})
	if got == "" {
		t.Fatal("action asing tak boleh menghasilkan kalimat kosong")
	}
	if !strings.Contains(got, "Budi") {
		t.Errorf("pelaku harus tetap disebut: %q", got)
	}
	// Kodenya WAJIB ikut — tanpa itu pembaca tahu ada sesuatu yang terjadi tapi
	// tak punya apa pun untuk dicari di kode sumber.
	if !strings.Contains(got, "sesuatu.yang.baru") {
		t.Errorf("action asing harus menyebut kodenya: %q", got)
	}
}

// TestSentence_PelakuTerhapusTakMenggantung: actor_user_id ON DELETE SET NULL,
// jadi keadaan ini pasti terjadi. Kalimat yang menggantung ("mengubah role Siti
// menjadi admin") membuat pembaca mengira ada data gagal dimuat, padahal yang
// terjadi adalah pelakunya memang sudah dihapus.
func TestSentence_PelakuTerhapusTakMenggantung(t *testing.T) {
	got := Sentence(Event{Action: "member.role.update", TargetUser: "Siti", Role: "admin"})
	if strings.HasPrefix(got, " ") || strings.HasPrefix(got, "mengubah") {
		t.Errorf("kalimat menggantung tanpa subjek: %q", got)
	}
	if !strings.Contains(got, "Seseorang") {
		t.Errorf("pelaku yang hilang harus disebut eksplisit: %q", got)
	}
}

// TestSentence_SasaranTerhapusTakMenggantung: target_id sengaja BUKAN FK justru
// agar jejak tetap terbaca setelah sasarannya hard-delete.
func TestSentence_SasaranTerhapusTakMenggantung(t *testing.T) {
	for _, c := range []struct{ action, wantKata string }{
		{"member.remove", "seorang user"},
		{"workspace.delete", "sebuah workspace"},
	} {
		got := Sentence(Event{Action: c.action, Actor: "Budi"})
		if !strings.Contains(got, c.wantKata) {
			t.Errorf("action %q: sasaran hilang harus disebut %q, got %q", c.action, c.wantKata, got)
		}
	}
}

// TestSentence_DetailMetadataIkutTampil: angka & alasan adalah justru bagian
// yang dicari saat menyelidiki. Kalau ia tercatat di DB tapi tak sampai ke
// layar, biaya penyimpanannya terbayar tanpa manfaat.
func TestSentence_DetailMetadataIkutTampil(t *testing.T) {
	cases := []struct {
		name string
		ev   Event
		want string
	}{
		{"role baru", Event{Action: "member.role.update", Actor: "A", TargetUser: "B", Role: "admin"}, "admin"},
		{"alasan suspend", Event{Action: "workspace.suspend", Actor: "A", Workspace: "Acme", Reason: "tunggakan"}, "tunggakan"},
		{"nilai kuota", Event{Action: "settings.workspace_quota", Actor: "A", Value: "5"}, "5"},
		{"metode login", Event{Action: "auth.login", Actor: "A", Method: "google"}, "google"},
		{"status baru", Event{Action: "user.status.update", Actor: "A", TargetUser: "B", Value: "blocked"}, "blocked"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Sentence(c.ev); !strings.Contains(got, c.want) {
				t.Errorf("kalimat harus memuat %q, got %q", c.want, got)
			}
		})
	}
}

// TestSentence_MetadataKosongTetapWajar: metadata boleh "{}" (banyak aksi tak
// menulisnya). Kalimatnya harus tetap wajar, bukan menyisakan " menjadi ."
func TestSentence_MetadataKosongTetapWajar(t *testing.T) {
	for _, act := range knownActions {
		got := Sentence(Event{Action: act, Actor: "Budi", TargetUser: "Siti", Workspace: "Acme"})
		for _, buruk := range []string{" .", "  ", "menjadi .", "alasan: ."} {
			if strings.Contains(got, buruk) {
				t.Errorf("action %q: kalimat cacat tanpa metadata (%q): %q", act, buruk, got)
			}
		}
	}
}

func TestFamily(t *testing.T) {
	cases := map[string]string{
		"workspace.create":   "workspace",
		"auth.login":         "auth",
		"member.role.update": "member",
		"tanpatitik":         "lainnya",
		"":                   "lainnya",
	}
	for in, want := range cases {
		if got := Family(in); got != want {
			t.Errorf("Family(%q) = %q, want %q", in, got, want)
		}
	}
}
