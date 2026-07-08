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
		// Slot toast (kanan-bawah), diisi via SSE patch (id "flash").
		h.Div(h.ID("flash"), h.Class("fixed bottom-4 right-4 z-50")),
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
					h.TBody(g.Map(rows, func(u UserRow) g.Node { return UserRowNode(u, canManageSuper) })),
				),
			),
		),
	)
}

// UserRowNode merender satu baris user. Di-export agar handler bisa me-render
// ulang baris ini via SSE setelah mutasi (id "user-<id>" jadi target morph).
func UserRowNode(u UserRow, canManageSuper bool) g.Node {
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

// roleControl = dropdown ubah role via Datastar SSE. Root env → badge (immutable).
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
	// @post {contentType:'form'} mencari FORM TERDEKAT lalu kirim nilainya, jadi
	// select WAJIB dibungkus <form> (kalau tidak, tak ada value terkirim).
	// Balasan SSE me-render ulang baris + toast (tanpa reload).
	return h.FormEl(
		h.Select(
			h.Class("input"),
			h.Name("role"),
			data.On("change", "@post('/dev/users/"+strconv.FormatInt(u.ID, 10)+"/role', {contentType: 'form'})"),
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

// statusControl = ubah status via Datastar SSE. Root env → badge (immutable).
func statusControl(u UserRow) g.Node {
	if u.IsRoot {
		return badge(u.Status, "")
	}
	return h.FormEl(
		h.Select(
			h.Class("input"),
			h.Name("status"),
			data.On("change", "@post('/dev/users/"+strconv.FormatInt(u.ID, 10)+"/status', {contentType: 'form'})"),
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

// deleteControl = tombol hapus (soft-delete) via SSE. Root env kebal → tanpa tombol.
func deleteControl(u UserRow) g.Node {
	if u.IsRoot {
		return g.Text("")
	}
	return h.Button(
		h.Type("button"),
		h.Class("btn"),
		g.Attr("data-variant", "destructive"),
		g.Attr("data-size", "sm"),
		data.On("click", "@post('/dev/users/"+strconv.FormatInt(u.ID, 10)+"/delete')"),
		g.Text("Hapus"),
	)
}

// Flash merender toast notifikasi (id "flash", target SSE patch). Auto-hilang
// via animasi CSS (.toast, fade-out) — TANPA inline script (patuh CSP
// script-src 'self'). ok=true → hijau (default alert), false → merah.
func Flash(ok bool, msg string) g.Node {
	inner := []g.Node{
		h.Class("alert toast shadow-lg"),
		h.Role("status"),
		g.Text(msg),
	}
	if !ok {
		inner = append(inner, g.Attr("data-variant", "destructive"))
	}
	return h.Div(
		h.ID("flash"),
		h.Class("fixed bottom-4 right-4 z-50"),
		h.Div(inner...),
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
