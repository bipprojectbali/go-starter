// Package pages berisi komponen halaman penuh (satu file per halaman).
package pages

import (
	"strconv"

	"go_stater/internal/db"
	"go_stater/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// TodoList merender halaman daftar todo dengan interaksi Datastar.
func TodoList(todos []db.Todo) g.Node {
	return h.Div(
		// Sinyal awal: field input $title. Datastar mengelola state klien.
		data.Signals(map[string]any{"title": ""}),

		h.H1(h.Class("text-xl font-semibold mb-4"), g.Text("Todos")),

		// Form tambah — tombol POST ke /todos via action Datastar.
		ui.Card(
			h.Div(
				h.Class("flex gap-2"),
				ui.Input(
					data.Bind("title"), // two-way bind ke sinyal $title
					h.Placeholder("Apa yang perlu dikerjakan?"),
				),
				ui.Button(
					ui.VariantDefault,
					[]g.Node{data.On("click", ui.PostAction("/todos"))},
					lucide.Plus(h.Class("size-4")),
					g.Text("Tambah"),
				),
			),
			// Slot alert error kosong — di-patch jadi Alert berisi saat validasi gagal.
			ui.AlertSlot("todo-error"),
		),

		// Daftar todo. id "todo-list" jadi target patch untuk item baru.
		h.Ul(
			h.ID("todo-list"),
			h.Class("list-none mt-4"),
			g.Map(todos, func(t db.Todo) g.Node { return TodoItem(t) }),
		),
	)
}

// TodoItem merender satu baris todo. id unik per item agar bisa di-patch/hapus
// individual via Datastar (mode outer butuh id).
func TodoItem(t db.Todo) g.Node {
	id := "todo-" + strconv.FormatInt(t.ID, 10)
	return h.Li(
		h.ID(id),
		h.Class("flex items-center justify-between gap-2 py-2 border-b border-base-300"),
		h.Span(g.Text(t.Title)),
		ui.Button(
			ui.VariantGhost,
			[]g.Node{
				data.On("click", ui.DeleteAction("/todos/"+strconv.FormatInt(t.ID, 10))),
				g.Attr("aria-label", "Hapus"),
			},
			lucide.Trash2(h.Class("size-4")),
		),
	)
}
