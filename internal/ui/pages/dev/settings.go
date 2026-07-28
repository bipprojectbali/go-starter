package dev

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// settings.go — pengaturan seluruh platform. Wadah umum, bukan halaman
// "kuota": pengaturan berikutnya (registrasi buka/tutup, TTL undangan, masa
// tenggang purge) punya rumah di sini tanpa menambah menu lagi.

// SettingsView = data siap-render halaman pengaturan platform.
type SettingsView struct {
	QuotaDefault int    // jatah workspace untuk user yang BELUM di-override
	QuotaMin     int    // batas bawah yang diterima backend
	QuotaMax     int    // batas atas
	OverrideN    int    // berapa user memegang hak khusus — konteks dampak
	Msg          string // pesan sukses
	Err          string // pesan galat
}

// Settings merender form pengaturan platform.
func Settings(v SettingsView) g.Node {
	body := []g.Node{
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Pengaturan Platform")),
		h.P(h.Class("text-base-content/70 mb-4"),
			g.Text("Aturan yang berlaku untuk seluruh platform. Perubahan langsung aktif tanpa restart.")),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "settings-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "settings-msg", g.Text(v.Msg)))
	}
	body = append(body, quotaCard(v))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// quotaCard = kuota workspace default. Form POST native → 303 (gotcha #16).
func quotaCard(v SettingsView) g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-1"), g.Text("Kuota Workspace")),
			h.P(h.Class("text-sm text-base-content/70 mb-3"),
				g.Text("Berapa workspace yang boleh DIMILIKI seorang user. "+
					"Diundang sebagai anggota workspace orang lain tidak memakan kuota.")),
			h.FormEl(
				h.Method("post"), h.Action("/dev/settings/quota"),
				h.Class("flex flex-wrap items-end gap-3 min-w-0"),
				h.Div(
					h.Class("grid gap-2"),
					ui.Label("Default global", h.For("quota")),
					ui.Input(
						h.ID("quota"), h.Name("quota"), h.Type("number"),
						h.Value(strconv.Itoa(v.QuotaDefault)),
						g.Attr("min", strconv.Itoa(v.QuotaMin)),
						g.Attr("max", strconv.Itoa(v.QuotaMax)),
						h.Required(),
						h.Class("input w-32"),
					),
				),
				h.Button(h.Type("submit"), h.Class("btn btn-primary"), g.Text("Simpan")),
			),
			// Dampak perubahan dinyatakan eksplisit: tanpa ini operator tak tahu
			// bahwa user ber-hak-khusus SENGAJA tak ikut berubah, lalu mengira
			// pengaturannya tak bekerja.
			h.P(h.Class("text-xs text-base-content/60 mt-3"),
				g.Text("Berlaku bagi semua user yang belum diberi hak khusus. "+
					overrideNote(v.OverrideN)+
					" Hak khusus per-user diatur di halaman Users.")),
		),
	)
}

// overrideNote menyusun kalimat jumlah pengecualian. Nol pengecualian ≠ kalimat
// kosong: "0 user" tetap informasi yang menenangkan saat operator ragu.
func overrideNote(n int) string {
	if n == 0 {
		return "Saat ini tidak ada user dengan hak khusus."
	}
	return strconv.Itoa(n) + " user memegang hak khusus dan TIDAK ikut berubah."
}
