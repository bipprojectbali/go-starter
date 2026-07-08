// Package dev berisi halaman panel developer (/dev).
package dev

import (
	"strconv"

	"go_stater/internal/ui"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// UserRow = data satu baris user untuk tabel (view model, bukan db.User).
type UserRow struct {
	ID        int64
	Email     string
	Role      string
	Status    string
	AvatarURL string
	IsRoot    bool // super-admin env (kontrol dinonaktifkan)
}

// UsersPage merender tabel user + kontrol role/status/hapus. canManageSuper =
// aktor boleh mengangkat super-admin (opsi super_admin muncul).
func UsersPage(rows []UserRow, canManageSuper bool) g.Node {
	return h.Div(
		h.H1(h.Class("text-xl font-semibold mb-4"), g.Text("Users")),
		h.Div(h.ID("flash")), // slot flash untuk error SSE (RequireEnforce/guard)
		h.Div(
			h.Class("card"),
			h.Section(
				h.Class("overflow-x-auto"),
				h.Table(
					h.Class("w-full text-sm"),
					h.THead(
						h.Tr(
							h.Class("border-b text-left text-muted-foreground"),
							th("User"), th("Role"), th("Status"), th("Aksi"),
						),
					),
					h.TBody(g.Map(rows, func(u UserRow) g.Node { return userRow(u, canManageSuper) })),
				),
			),
		),
	)
}

func userRow(u UserRow, canManageSuper bool) g.Node {
	rowID := "user-" + strconv.FormatInt(u.ID, 10)
	return h.Tr(
		h.ID(rowID),
		h.Class("border-b"),
		// Kolom user: avatar + email (+ badge root).
		h.Td(
			h.Class("py-2 pr-4"),
			h.Div(
				h.Class("flex items-center gap-2"),
				ui.Avatar(u.AvatarURL, "", u.Email, 32),
				h.Span(g.Text(u.Email)),
				ui.When(u.IsRoot, badge("root", "outline")),
			),
		),
		h.Td(h.Class("py-2 pr-4"), roleControl(u, canManageSuper)),
		h.Td(h.Class("py-2 pr-4"), statusControl(u)),
		h.Td(h.Class("py-2"), deleteControl(u)),
	)
}

// roleControl = dropdown ubah role. Dinonaktifkan untuk root env.
func roleControl(u UserRow, canManageSuper bool) g.Node {
	if u.IsRoot {
		return badge(u.Role, "")
	}
	opts := []g.Node{
		roleOption("user", u.Role),
		roleOption("admin", u.Role),
	}
	if canManageSuper {
		opts = append(opts, roleOption("super_admin", u.Role))
	}
	return h.Form(
		h.Method("post"),
		h.Action("/dev/users/"+strconv.FormatInt(u.ID, 10)+"/role"),
		h.Select(
			h.Class("input"),
			h.Name("role"),
			data.On("change", "el.form.requestSubmit()"),
			g.Group(opts),
		),
	)
}

func roleOption(val, current string) g.Node {
	attrs := []g.Node{h.Value(val)}
	if val == current {
		attrs = append(attrs, h.Selected())
	}
	return h.Option(append(attrs, g.Text(val))...)
}

// statusControl = tombol aktif/nonaktif/blokir. Root env → badge saja.
func statusControl(u UserRow) g.Node {
	if u.IsRoot {
		return badge(u.Status, "")
	}
	return h.Form(
		h.Method("post"),
		h.Action("/dev/users/"+strconv.FormatInt(u.ID, 10)+"/status"),
		h.Select(
			h.Class("input"),
			h.Name("status"),
			data.On("change", "el.form.requestSubmit()"),
			statusOption("active", u.Status),
			statusOption("disabled", u.Status),
			statusOption("blocked", u.Status),
		),
	)
}

func statusOption(val, current string) g.Node {
	attrs := []g.Node{h.Value(val)}
	if val == current {
		attrs = append(attrs, h.Selected())
	}
	return h.Option(append(attrs, g.Text(val))...)
}

// deleteControl = tombol hapus (soft-delete). Root env kebal → tanpa tombol.
func deleteControl(u UserRow) g.Node {
	if u.IsRoot {
		return g.Text("")
	}
	return h.Form(
		h.Method("post"),
		h.Action("/dev/users/"+strconv.FormatInt(u.ID, 10)+"/delete"),
		h.Button(
			h.Type("submit"),
			h.Class("btn"),
			g.Attr("data-variant", "destructive"),
			g.Attr("data-size", "sm"),
			g.Text("Hapus"),
		),
	)
}

func th(label string) g.Node {
	return h.Th(h.Class("py-2 pr-4 font-medium"), g.Text(label))
}

func badge(text, variant string) g.Node {
	attrs := []g.Node{h.Class("badge")}
	if variant != "" {
		attrs = append(attrs, g.Attr("data-variant", variant))
	}
	return h.Span(append(attrs, g.Text(text))...)
}
