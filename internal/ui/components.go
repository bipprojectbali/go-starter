package ui

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

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

// Card membungkus konten dalam kartu Basecoat. Basecoat memberi padding
// horizontal & spacing lewat `.card > section` (bukan langsung ke `.card`),
// jadi isi WAJIB dibungkus <section> — kalau anak ditaruh langsung di .card,
// konten mepet dan tiap anak kena gap 24px kartu (spacing dobel).
func Card(children ...g.Node) g.Node {
	return h.Div(
		h.Class("card"),
		h.Section(children...),
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
