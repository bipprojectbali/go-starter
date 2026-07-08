package ui

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// When merender node hanya bila allowed; selainnya node kosong. Untuk render
// kondisional berbasis izin (mis. tombol admin) — flag di-PRECOMPUTE di handler
// (authz.Can), bukan dipanggil dari dalam fungsi gomponents. Ini kosmetik UI;
// pertahanan sebenarnya tetap di middleware/service.
func When(allowed bool, node g.Node) g.Node {
	if allowed {
		return node
	}
	return g.Text("")
}

// Variant adalah tipe typed untuk varian komponen — typo jadi compile error,
// bukan string kosong senyap (§4.5).
type Variant int

const (
	VariantDefault Variant = iota
	VariantDestructive
	VariantOutline
	VariantGhost
)

// btnVariant memetakan varian ke nilai data-variant Basecoat v1.x.
// Basecoat memakai class root "btn" + atribut data-variant (BUKAN class
// "btn-destructive"). Array indeks enum: menambah Variant tanpa entri = compile error.
var btnVariant = [...]string{
	VariantDefault:     "", // default = primary, tanpa data-variant
	VariantDestructive: "destructive",
	VariantOutline:     "outline",
	VariantGhost:       "ghost",
}

// Button merender tombol Basecoat: class "btn" + data-variant sesuai varian.
// Atribut tambahan (mis. data-on Datastar) dilewatkan lewat attrs.
func Button(variant Variant, attrs []g.Node, children ...g.Node) g.Node {
	base := []g.Node{h.Class("btn")}
	if v := btnVariant[variant]; v != "" {
		base = append(base, g.Attr("data-variant", v))
	}
	return h.Button(append(base, append(attrs, g.Group(children))...)...)
}

// Card membungkus konten dalam kartu Basecoat. `.card` memberi gap 24px hanya
// antar ANAK LANGSUNGnya; padding horizontal datang dari `.card > section`.
// Karena semua isi dibungkus SATU <section>, gap `.card` tak berlaku (cuma 1
// anak) — jadi <section> sendiri harus jadi grid ber-gap agar field/tombol di
// dalamnya tidak menempel. gap-6 (24px) menyamai spacing kartu Basecoat.
func Card(children ...g.Node) g.Node {
	return h.Div(
		h.Class("card"),
		h.Section(append([]g.Node{h.Class("grid gap-6")}, children...)...),
	)
}

// Input adalah field teks Basecoat. attrs untuk data-bind, placeholder, dll.
func Input(attrs ...g.Node) g.Node {
	return h.Input(append([]g.Node{h.Class("input"), h.Type("text")}, attrs...)...)
}

// Label untuk field form.
func Label(text string, attrs ...g.Node) g.Node {
	return h.Label(append([]g.Node{h.Class("label")}, append(attrs, g.Text(text))...)...)
}

// AlertSlot merender wadah error KOSONG (tanpa class .alert) sebagai target
// patch Datastar. Penting: `.alert` Basecoat selalu punya border 1px, jadi
// merender .alert saat kosong menghasilkan kotak border hantu. Slot ini hanya
// <div id> kosong; isi diganti Alert() lengkap saat server mem-patch error.
func AlertSlot(id string) g.Node {
	return h.Div(h.ID(id))
}

// Alert menampilkan pesan (mis. error validasi). Basecoat: class "alert" +
// data-variant="destructive". Punya id (sama dengan slot) agar patch outer
// menggantikan slot kosong dengan alert berisi.
func Alert(variant Variant, id string, children ...g.Node) g.Node {
	attrs := []g.Node{h.ID(id), h.Class("alert"), h.Role("alert")}
	if variant == VariantDestructive {
		attrs = append(attrs, g.Attr("data-variant", "destructive"))
	}
	return h.Div(append(attrs, g.Group(children))...)
}
