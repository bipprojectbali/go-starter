package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// InviteResult merender halaman undangan publik (/invite/{token}): konfirmasi
// bergabung bila valid, atau pesan kesalahan (kedaluwarsa/terpakai/tak ditemukan).
// Form NATIVE POST → 303 (gotcha #16: redirect via SSE diblokir CSP).
func InviteResult(workspaceName, errMsg string, canAccept bool) g.Node {
	if !canAccept {
		return h.Div(
			h.Class("mx-auto w-full max-w-md"),
			h.H1(h.Class("text-xl font-semibold mb-4"), g.Text("Undangan")),
			ui.Card(
				ui.Alert(ui.VariantDestructive, "invite-err", g.Text(errMsg)),
				h.A(h.Href("/"), h.Class("btn btn-outline w-full"), g.Text("Ke Beranda")),
			),
		)
	}
	return h.Div(
		h.Class("mx-auto w-full max-w-md"),
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Undangan Workspace")),
		h.P(h.Class("text-base-content/70 mb-4"),
			g.Text("Anda diundang bergabung ke workspace berikut.")),
		ui.Card(
			h.P(h.Class("text-lg font-semibold"), g.Text(workspaceName)),
			// Token diambil dari URL saat ini oleh handler; form post ke path yang sama + /accept.
			h.FormEl(
				h.Method("post"), h.Action("accept"),
				h.Button(h.Type("submit"), h.Class("btn btn-primary w-full"), g.Text("Gabung")),
			),
			h.A(h.Href("/"), h.Class("btn btn-ghost w-full"), g.Text("Nanti saja")),
		),
	)
}
