package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// NotifInviteRow = satu undangan masuk yang menunggu keputusan.
type NotifInviteRow struct {
	Token     string
	Workspace string
	Role      string
	Expires   string
}

// NotifRow = satu peristiwa (role diubah, dikeluarkan, dst) — hanya dibaca.
type NotifRow struct {
	Text      string
	Workspace string
	When      string
	Unread    bool
}

// Notifications merender umpan notifikasi user. Undangan DI ATAS karena butuh
// tindakan; peristiwa di bawah karena hanya perlu dibaca. Keduanya kosong →
// pesan kosong yang jujur, bukan halaman hampa.
func Notifications(invites []NotifInviteRow, events []NotifRow, errMsg string) g.Node {
	body := []g.Node{
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Notifikasi")),
		h.P(h.Class("text-base-content/70 mb-4"),
			g.Text("Undangan workspace dan kabar keanggotaan Anda.")),
	}
	if errMsg != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "notif-err", g.Text(errMsg)))
	}
	if len(invites) > 0 {
		body = append(body, inviteInbox(invites))
	}
	if len(events) > 0 {
		body = append(body, eventList(events))
	}
	if len(invites) == 0 && len(events) == 0 {
		body = append(body, emptyNotif())
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// inviteInbox = undangan menunggu keputusan. Terima/Tolak = form NATIVE POST →
// 303 (gotcha #16: redirect lewat SSE diblokir CSP).
func inviteInbox(invites []NotifInviteRow) g.Node {
	cards := make([]g.Node, 0, len(invites))
	for _, i := range invites {
		cards = append(cards, inviteCard(i))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0 gap-3"),
			h.H2(h.Class("font-semibold"), g.Text("Undangan Masuk")),
			g.Group(cards),
		),
	)
}

func inviteCard(i NotifInviteRow) g.Node {
	return h.Div(
		// Mobile-first: tumpuk vertikal, sejajar mulai sm (konvensi mobile-first).
		h.Class("flex flex-col gap-3 rounded-md border border-base-300 p-3 "+
			"sm:flex-row sm:items-center sm:justify-between"),
		h.Div(
			h.Class("min-w-0"),
			h.P(h.Class("font-medium break-words"),
				g.Text("Bergabung ke "+i.Workspace)),
			h.P(h.Class("text-sm text-base-content/70"),
				g.Text("Sebagai "+i.Role+" · berlaku s/d "+i.Expires)),
		),
		h.Div(
			h.Class("flex gap-2 shrink-0"),
			h.FormEl(
				h.Method("post"), h.Action("/notifications/invite/"+i.Token+"/accept"),
				h.Button(h.Type("submit"), h.Class("btn btn-primary btn-sm"),
					g.Text("Terima")),
			),
			h.FormEl(
				h.Method("post"), h.Action("/notifications/invite/"+i.Token+"/decline"),
				h.Button(h.Type("submit"), h.Class("btn btn-ghost btn-sm"),
					g.Text("Tolak")),
			),
		),
	)
}

// eventList = kabar yang sudah lewat. Yang belum terbaca diberi aksen kiri —
// bukan warna teks, agar tetap terbaca di semua tema (gotcha #11).
func eventList(events []NotifRow) g.Node {
	rows := make([]g.Node, 0, len(events))
	for _, e := range events {
		cls := "flex flex-col gap-1 border-l-2 border-transparent pl-3 py-2 " +
			"sm:flex-row sm:items-baseline sm:justify-between sm:gap-4"
		if e.Unread {
			cls += " border-primary"
		}
		rows = append(rows, h.Div(
			h.Class(cls),
			h.P(h.Class("min-w-0 break-words"), g.Text(e.Text)),
			h.Span(h.Class("text-xs text-base-content/60 shrink-0"), g.Text(e.When)),
		))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0 gap-1"),
			h.H2(h.Class("font-semibold mb-1"), g.Text("Kabar Terbaru")),
			g.Group(rows),
		),
	)
}

func emptyNotif() g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(
			h.Class("card-body items-center text-center gap-1 py-10"),
			h.P(h.Class("font-medium"), g.Text("Belum ada notifikasi")),
			h.P(h.Class("text-sm text-base-content/70"),
				g.Text("Undangan workspace dan kabar keanggotaan akan muncul di sini.")),
		),
	)
}
