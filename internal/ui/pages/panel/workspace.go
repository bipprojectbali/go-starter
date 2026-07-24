package panel

import (
	"go_stater/internal/ui"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// Workspace merender halaman pengaturan workspace: form ganti NAMA tampilan.
// canEdit=false (mis. admin non-owner) → field disabled + tombol hilang, hanya
// lihat. name = nama saat ini (mengisi signal). slug ditampilkan read-only
// (identitas immutable — konteks bagi user, bukan editable).
func Workspace(name, slug string, canEdit bool) g.Node {
	fields := []g.Node{
		h.Div(
			h.Class("grid gap-2"),
			ui.Label("Nama Workspace", h.For("name")),
			ui.Input(
				h.ID("name"),
				h.Type("text"),
				data.Bind("name"),
				h.Placeholder("mis. Acme Corp"),
				g.If(!canEdit, h.Disabled()),
			),
		),
		h.Div(
			h.Class("grid gap-2"),
			ui.Label("Slug (identitas, tak bisa diubah)", h.For("slug")),
			ui.Input(h.ID("slug"), h.Type("text"), h.Value(slug), h.Disabled()),
		),
	}
	if canEdit {
		fields = append(fields,
			ui.Button(
				ui.VariantDefault,
				[]g.Node{data.On("click", ui.PostAction("/admin/workspace"))},
				g.Text("Simpan"),
			),
			ui.AlertSlot("workspace-msg"),
		)
	} else {
		fields = append(fields, h.P(
			h.Class("text-sm text-base-content/60"),
			g.Text("Hanya owner yang dapat mengubah nama workspace."),
		))
	}

	return h.Div(
		data.Signals(map[string]any{"name": name}),
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Pengaturan Workspace")),
		h.P(h.Class("text-base-content/70 mb-4"), g.Text("Kelola nama workspace Anda.")),
		ui.Card(fields...),
	)
}
