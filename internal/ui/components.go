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

// btnVariant memetakan varian ke modifier Basecoat. Array indeks enum, bukan
// map string: menambah Variant tanpa entri di sini = compile error (array
// literal wajib lengkap bila diindeks konstanta).
var btnVariant = [...]string{
	VariantDefault:     "",
	VariantDestructive: "btn-destructive",
	VariantOutline:     "btn-outline",
	VariantGhost:       "btn-ghost",
}

// Button merender tombol dengan class Basecoat "btn" + modifier varian.
// Atribut tambahan (mis. data-on Datastar) dilewatkan lewat attrs.
func Button(variant Variant, attrs []g.Node, children ...g.Node) g.Node {
	return h.Button(
		append([]g.Node{h.Class("btn " + btnVariant[variant])}, append(attrs, g.Group(children))...)...,
	)
}

// Card membungkus konten dalam kartu Basecoat.
func Card(children ...g.Node) g.Node {
	return h.Div(append([]g.Node{h.Class("card")}, children...)...)
}

// Input adalah field teks Basecoat. attrs untuk data-bind, placeholder, dll.
func Input(attrs ...g.Node) g.Node {
	return h.Input(append([]g.Node{h.Class("input"), h.Type("text")}, attrs...)...)
}

// Label untuk field form.
func Label(text string, attrs ...g.Node) g.Node {
	return h.Label(append([]g.Node{h.Class("label")}, append(attrs, g.Text(text))...)...)
}

// alertVariant memetakan varian ke modifier alert Basecoat.
var alertVariant = [...]string{
	VariantDefault:     "",
	VariantDestructive: "alert-destructive",
	VariantOutline:     "",
	VariantGhost:       "",
}

// Alert menampilkan pesan (mis. error validasi). Punya id agar bisa di-patch
// via Datastar (mode default outer butuh id).
func Alert(variant Variant, id string, children ...g.Node) g.Node {
	return h.Div(
		h.ID(id),
		h.Class("alert "+alertVariant[variant]),
		h.Role("alert"),
		g.Group(children),
	)
}
