package ui

import (
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// dsx = helper Datastar bertipe. Tujuan: memindah footgun authoring "JS-sebagai-
// string di atribut" dari gagal-senyap-runtime jadi mustahil-ditulis. CHARTER
// (jaga tetap tipis, agent-friendly): tiap helper = SATU jaminan quoting/co-render,
// TANPA logika/DSL. Runtime tetap Datastar (unsafe-eval tetap perlu — itu di luar
// jangkauan generasi string server-side; lihat CLAUDE.md gotcha CSP). Helper ini
// menutup gotcha #5 (data-class hyphen) & #6 (@post butuh <form>) secara struktural.

// ClassOn men-toggle SATU class berdasar ekspresi Datastar. Nama class di-passing
// TELANJANG (tanpa kutip); helper yang mengutipnya. Ini membuat gotcha #5
// (`data-class` key ber-hyphen wajib di-quote) MUSTAHIL ditulis salah: tak ada
// jalur untuk memberi key tanpa kutip. Emit identik dengan
// data.Class("'<class>'", expr) → data-class="{'<class>': <expr>}".
func ClassOn(class, expr string) g.Node {
	return data.Class("'"+class+"'", expr)
}

// Classes = ClassOn untuk banyak class sekaligus. Sama jaminannya (tiap key
// dikutip helper), tanpa peluang lupa mengutip salah satu.
type ClassRule struct {
	Class string // nama class TELANJANG (tanpa kutip)
	Expr  string // ekspresi Datastar (mis. "$open")
}

func Classes(rules ...ClassRule) g.Node {
	pairs := make([]string, 0, len(rules)*2)
	for _, r := range rules {
		pairs = append(pairs, "'"+r.Class+"'", r.Expr)
	}
	return data.Class(pairs...)
}

// PostAction / DeleteAction membangun ekspresi aksi Datastar bertipe. Menghapus
// juggle-kutip manual di call site (auth.go/components.go). Catatan jujur:
// ini MEMPERBAIKI (bukan mustahilkan) site string — hasilnya tetap string, tapi
// bentuknya dijamin benar. Untuk form-valued post, pakai FormPostSelect.
func PostAction(url string) string   { return "@post('" + url + "')" }
func DeleteAction(url string) string { return "@delete('" + url + "')" }

// FormPostSelect merender <select> yang mem-@post nilainya BESERTA <form>
// pembungkusnya sebagai satu node. Ini membuat gotcha #6 (@post {contentType:'form'}
// butuh <form> terdekat) MUSTAHIL salah: aksi form-valued tak punya jalur lain
// selain lewat helper ini, jadi tak mungkin ada select-post tanpa form-nya. String
// contentType juga dibangun internal → typo ('from') mustahil. Emit identik dengan
// pola lama: <form><select class="select select-sm" name=... data-on:change="@post(...,
// {contentType: 'form'})">opts...</select></form>.
func FormPostSelect(postURL, name string, opts ...g.Node) g.Node {
	attrs := []g.Node{
		h.Class("select select-sm"),
		h.Name(name),
		data.On("change", "@post('"+postURL+"', {contentType: 'form'})"),
	}
	return h.FormEl(h.Select(append(attrs, opts...)...))
}
