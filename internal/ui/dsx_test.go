package ui

import (
	"strings"
	"testing"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// TestClassOn_QuotesHyphenatedKey: ClassOn mengutip nama class → key ber-hyphen
// yang VALID (gotcha #5). Bukti byte-identik dengan pola lama data.Class("'x'", …):
// output harus `data-class="{&#39;translate-x-0&#39;: $sidebarOpen}"`.
func TestClassOn_QuotesHyphenatedKey(t *testing.T) {
	out := renderAttr(t, ClassOn("translate-x-0", "$sidebarOpen"))
	if !strings.Contains(out, `{&#39;translate-x-0&#39;: $sidebarOpen}`) {
		t.Errorf("ClassOn harus mengutip key ber-hyphen:\n%s", out)
	}
	// Grep-ability by contract: atribut data-class tetap literal di output.
	if !strings.Contains(out, "data-class=") {
		t.Errorf("output harus tetap atribut data-class (grep-able):\n%s", out)
	}
}

// TestClasses_QuotesEachKey: varian multi-class mengutip TIAP key.
func TestClasses_QuotesEachKey(t *testing.T) {
	out := renderAttr(t, Classes(
		ClassRule{Class: "is-open", Expr: "$a"},
		ClassRule{Class: "translate-x-0", Expr: "$b"},
	))
	for _, want := range []string{`&#39;is-open&#39;: $a`, `&#39;translate-x-0&#39;: $b`} {
		if !strings.Contains(out, want) {
			t.Errorf("Classes kurang %q:\n%s", want, out)
		}
	}
}

// TestPostAction_DeleteAction: konstruktor aksi bertipe menghasilkan ekspresi
// Datastar yang benar (grep-able literal @post/@delete).
func TestPostAction_DeleteAction(t *testing.T) {
	if got := PostAction("/logout"); got != "@post('/logout')" {
		t.Errorf("PostAction salah: %q", got)
	}
	if got := DeleteAction("/todos/42"); got != "@delete('/todos/42')" {
		t.Errorf("DeleteAction salah: %q", got)
	}
}

// TestFormPostSelect_WrapsInForm: @post {contentType:'form'} SELALU dibungkus
// <form> (gotcha #6 mustahil salah). Output byte-identik dengan pola lama:
// <form><select class="select select-sm" name="role" data-on:change="@post('/x/role', {contentType: 'form'})">…
func TestFormPostSelect_WrapsInForm(t *testing.T) {
	out := renderNode(t, FormPostSelect("/dev/users/7/role", "role", h.Option(g.Text("user"))))

	if !strings.Contains(out, `<form><select class="select select-sm" name="role"`) {
		t.Errorf("select harus dibungkus <form> dengan class benar:\n%s", out)
	}
	if !strings.Contains(out, `@post(&#39;/dev/users/7/role&#39;, {contentType: &#39;form&#39;})`) {
		t.Errorf("aksi form-post harus benar (contentType:form):\n%s", out)
	}
	// Wrapper <form> menutup — struktur lengkap.
	if !strings.Contains(out, "</select></form>") {
		t.Errorf("form harus membungkus select penuh:\n%s", out)
	}
}

// renderAttr merender node atribut tunggal dengan membungkusnya di <div> agar
// atribut ikut ter-render (atribut telanjang tak bisa Render sendiri sbg elemen).
func renderAttr(t *testing.T, attr g.Node) string {
	t.Helper()
	return renderNode(t, h.Div(attr))
}
