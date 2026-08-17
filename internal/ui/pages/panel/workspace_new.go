package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// WorkspaceNew merender form buat workspace baru. Dipakai dua konteks: user yang
// ingin menambah workspace (dari switcher), DAN user yang belum punya workspace
// sama sekali (diarahkan middleware Scope) — teks bantuannya sama-sama masuk akal.
// Form NATIVE POST → 303 (gotcha #16: redirect via SSE diblokir CSP).
func WorkspaceNew(errMsg string) g.Node {
	card := []g.Node{}
	if errMsg != "" {
		card = append(card, ui.Alert(ui.VariantDestructive, "workspace-err", g.Text(errMsg)))
	}
	card = append(card,
		h.FormEl(
			h.Method("post"), h.Action("/workspace/new"), h.Class("grid gap-4"),
			h.Div(
				h.Class("grid gap-2"),
				ui.Label("Nama Workspace", h.For("name")),
				ui.Input(
					h.ID("name"), h.Name("name"), h.Type("text"),
					h.Placeholder("mis. Acme Corp"), h.AutoFocus(),
				),
			),
			h.Button(h.Type("submit"), h.Class("btn btn-primary w-full"), g.Text("Buat Workspace")),
		),
	)
	return h.Div(
		h.Class("mx-auto w-full max-w-md"),
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Workspace Baru")),
		h.P(h.Class("text-base-content/80 mb-4"),
			g.Text("Buat ruang kerja baru. Anda otomatis jadi owner-nya.")),
		ui.Card(card...),
	)
}
