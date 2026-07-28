package ui

// panelkind.go — identitas visual per panel (/user, /admin, /dev).
//
// Kenapa perlu: ketiga panel memakai AppShell yang sama, dan satu-satunya
// pembeda dulu adalah sub-label abu kecil (`go_starter /dev`) — praktis tak
// terbaca sekilas, dan /user tak punya sufiks sama sekali. Akibatnya user tak
// tahu sedang di shell mana. Ini BUKAN sekadar kosmetik: /dev menampilkan data
// LINTAS-workspace (ListUsers melihat semua tenant), jadi salah mengira sedang
// di /admin = salah membaca cakupan data.
//
// Dua penanda sekaligus, sengaja: chip TEKS (tetap terbaca bila warna tak
// tertangkap mata — sekitar 1 dari 12 pria buta warna) + AKSEN warna (tertangkap
// tanpa membaca). Salah satunya saja meninggalkan sebagian orang.

// Panel = jenis panel yang sedang dibuka. Tipe bertipe (bukan string bebas):
// nilai ngawur jadi compile error, bukan chip kosong senyap.
type Panel int

const (
	PanelNone  Panel = iota // landing/halaman tanpa identitas panel
	PanelUser               // ruang kerja anggota
	PanelAdmin              // pengelolaan satu workspace
	PanelDev                // platform: LINTAS-workspace
)

// panelStyle = tampilan satu panel. Warna memakai token semantik daisyUI
// (primary/secondary/warning), BUKAN warna absolut seperti bg-red-500 — token
// didefinisikan ulang oleh tiap tema sehingga aksen ikut menyesuaikan di ke-6
// tema (gotcha #11: warna absolut tak adaptif antar-tema).
type panelStyle struct {
	Label string // teks chip, huruf besar & pendek
	Chip  string // class chip daisyUI
	Edge  string // class garis aksen di tepi atas sidebar
}

// panelStyles diindeks Panel — menambah Panel tanpa entri = compile error
// (array berukuran tetap), jadi mustahil lupa mendaftarkan gayanya.
var panelStyles = [...]panelStyle{
	PanelNone:  {},
	PanelUser:  {Label: "RUANG KERJA", Chip: "badge-primary", Edge: "bg-primary"},
	PanelAdmin: {Label: "ADMIN", Chip: "badge-secondary", Edge: "bg-secondary"},
	// /dev pakai `warning`: bukan sekadar warna ketiga, tapi peringatan — di sini
	// data lintas-tenant terlihat dan tindakan berdampak ke SEMUA workspace.
	PanelDev: {Label: "PLATFORM", Chip: "badge-warning", Edge: "bg-warning"},
}

// PanelIdentity membuka gaya panel (label, class chip, class aksen) untuk
// pemeriksaan dari luar paket — dipakai test yang menjaga ketiga panel tetap
// berbeda satu sama lain. Bukan untuk render; view memakai panelChip/panelEdge.
func PanelIdentity(p Panel) (label, chip, edge string) {
	st := p.style()
	return st.Label, st.Chip, st.Edge
}

// style mengembalikan gaya panel; PanelNone → zero value (tak dirender).
func (p Panel) style() panelStyle {
	if int(p) < 0 || int(p) >= len(panelStyles) {
		return panelStyle{}
	}
	return panelStyles[p]
}
