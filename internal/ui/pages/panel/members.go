package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// MemberRow = satu anggota workspace (view).
type MemberRow struct {
	UserID    int64
	Email     string
	Role      string
	AvatarURL string
	Status    string
}

// InviteRow = satu undangan pending. Link ditampilkan untuk DISALIN manual —
// pengiriman email belum ada (task terbuka, lihat handler/invite.go).
type InviteRow struct {
	ID      int64
	Email   string
	Role    string
	Link    string
	Expires string
}

// Members merender panel anggota workspace: daftar anggota (ubah role/keluarkan),
// form undang, dan daftar undangan pending. canManage=false → hanya lihat.
// selfID dipakai menyembunyikan aksi terhadap diri sendiri.
//
// base = prefix URL workspace ini (mis. "/w/acme"), DIOPER dari handler — view
// tak boleh merakit path sendiri: sejak 0004 setiap aksi bergantung slug, dan
// path yang di-hardcode di view akan diam-diam menunjuk workspace yang salah.
func Members(base string, members []MemberRow, invites []InviteRow, canManage bool, selfID int64, errMsg string) g.Node {
	body := []g.Node{
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Anggota Workspace")),
		h.P(h.Class("text-base-content/70 mb-4"),
			g.Text("Kelola siapa saja yang punya akses ke workspace ini.")),
	}
	if errMsg != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "members-err", g.Text(errMsg)))
	}
	if canManage {
		body = append(body, inviteForm(base))
	}
	body = append(body, memberList(base, members, canManage, selfID))
	if canManage && len(invites) > 0 {
		body = append(body, inviteList(base, invites))
	}
	// min-w-0 di wadah grid: tanpa ini, tabel/input lebar di dalamnya memaksa
	// grid melebar → halaman overflow horizontal di mobile (konvensi mobile-first).
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// memberList = tabel anggota. Tabel dibungkus ui.TableScroll agar scroll-nya
// terkurung, tak mendorong lebar halaman di mobile (konvensi mobile-first).
func memberList(base string, members []MemberRow, canManage bool, selfID int64) g.Node {
	rows := make([]g.Node, 0, len(members))
	for _, m := range members {
		rows = append(rows, memberRow(base, m, canManage, selfID))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-2"), g.Text("Anggota")),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Email")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Role")),
					h.Th(h.Class("py-2 font-medium"), g.Text("")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func memberRow(base string, m MemberRow, canManage bool, selfID int64) g.Node {
	id := strconv.FormatInt(m.UserID, 10)
	roleCell := g.Node(h.Span(h.Class("badge badge-neutral"), g.Text(m.Role)))
	action := g.Node(g.Text(""))
	// Aksi hanya untuk pengelola, dan tak pernah terhadap diri sendiri (cegah
	// mengunci diri keluar dari workspace sendiri).
	if canManage && m.UserID != selfID {
		roleCell = h.FormEl(
			h.Method("post"), h.Action(base+"/members/"+id+"/role"),
			h.Class("flex items-center gap-2"),
			h.Select(
				h.Class("select select-sm"), h.Name("role"),
				roleOpt("member", m.Role), roleOpt("admin", m.Role), roleOpt("owner", m.Role),
			),
			h.Button(h.Type("submit"), h.Class("btn btn-sm"), g.Text("Simpan")),
		)
		action = h.FormEl(
			h.Method("post"), h.Action(base+"/members/"+id+"/remove"),
			h.Button(h.Type("submit"), h.Class("btn btn-sm btn-error btn-outline"),
				g.Text("Keluarkan")),
		)
	}
	return h.Tr(
		h.Class("border-b border-base-300/50"),
		h.Td(h.Class("py-2 pr-4"), h.Div(
			h.Class("flex items-center gap-2 min-w-0"),
			ui.Avatar(m.AvatarURL, "", m.Email, 32),
			h.Span(h.Class("truncate"), g.Text(m.Email)),
		)),
		h.Td(h.Class("py-2 pr-4"), roleCell),
		h.Td(h.Class("py-2"), action),
	)
}

func roleOpt(val, current string) g.Node {
	attrs := []g.Node{h.Value(val)}
	if val == current {
		attrs = append(attrs, h.Selected())
	}
	return h.Option(append(attrs, g.Text(val))...)
}

// inviteForm = undang lewat email. Form NATIVE POST → 303 (gotcha #16).
func inviteForm(base string) g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(
			h.Class("card-body"),
			h.H2(h.Class("font-semibold mb-2"), g.Text("Undang Anggota")),
			h.FormEl(
				h.Method("post"), h.Action(base+"/members/invite"),
				h.Class("flex flex-col gap-2 sm:flex-row sm:items-end"),
				h.Div(
					h.Class("grid gap-2 flex-1"),
					ui.Label("Email", h.For("invite-email")),
					ui.Input(h.ID("invite-email"), h.Name("email"), h.Type("email"),
						h.Placeholder("nama@contoh.com")),
				),
				h.Div(
					h.Class("grid gap-2"),
					ui.Label("Role", h.For("invite-role")),
					h.Select(h.ID("invite-role"), h.Class("select"), h.Name("role"),
						h.Option(h.Value("member"), g.Text("member")),
						h.Option(h.Value("admin"), g.Text("admin")),
					),
				),
				h.Button(h.Type("submit"), h.Class("btn btn-primary"), g.Text("Undang")),
			),
			h.P(h.Class("text-xs text-base-content/60 mt-2"),
				g.Text("Email belum dikirim otomatis — salin tautan undangan di bawah dan kirim manual.")),
		),
	)
}

// inviteList = undangan pending + tautannya (untuk disalin manual).
func inviteList(base string, invites []InviteRow) g.Node {
	rows := make([]g.Node, 0, len(invites))
	for _, i := range invites {
		rows = append(rows, h.Tr(
			h.Class("border-b border-base-300/50"),
			h.Td(h.Class("py-2 pr-4"), g.Text(i.Email)),
			h.Td(h.Class("py-2 pr-4"), h.Span(h.Class("badge badge-neutral"), g.Text(i.Role))),
			h.Td(h.Class("py-2 pr-4"), ui.Input(
				h.Type("text"), h.Value(i.Link), h.ReadOnly(),
				h.Class("input input-sm w-full min-w-[16rem] font-mono text-xs"),
			)),
			h.Td(h.Class("py-2 pr-4 text-xs text-base-content/60"), g.Text(i.Expires)),
			h.Td(h.Class("py-2"), h.FormEl(
				h.Method("post"),
				h.Action(base+"/members/invite/"+strconv.FormatInt(i.ID, 10)+"/delete"),
				h.Button(h.Type("submit"), h.Class("btn btn-sm btn-ghost"), g.Text("Batal")),
			)),
		))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-2"), g.Text("Undangan Menunggu")),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Email")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Role")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Tautan")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Berlaku s/d")),
					h.Th(h.Class("py-2 font-medium"), g.Text("")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}
