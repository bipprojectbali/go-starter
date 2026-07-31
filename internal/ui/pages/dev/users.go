// Package dev berisi halaman panel developer (/dev).
package dev

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// UserRow = data satu baris user untuk tabel (view model, bukan db.User).
type UserRow struct {
	ID    int64
	Email string
	// Name = nama tampilan dari provider; "" bila tak diketahui (akun password
	// dev, atau Google tak mengirimkannya). Di panel platform email TETAP tampil
	// utuh di samping nama — operator platform memakainya untuk mencocokkan
	// orang, dan nama tidak unik.
	Name          string
	Status        string
	AvatarURL     string
	IsRoot        bool            // super-admin env (kontrol dinonaktifkan)
	Workspaces    []WorkspaceRole // keanggotaan: role PER-workspace (model membership)
	Quota         int             // kuota EFEKTIF (override bila ada, selainnya default global)
	QuotaOverride bool            // true = angka di atas hak KHUSUS user ini, bukan warisan global
}

// WorkspaceRole = satu keanggotaan user (workspace + role di dalamnya). Role kini
// per-workspace, jadi panel platform menampilkan/mengubah per baris ini.
type WorkspaceRole struct {
	TenantID int64
	Name     string
	Role     string
}

// UsersPage merender tabel user + kontrol role/status/hapus. canManageSuper =
// aktor boleh mengangkat super-admin (opsi super_admin muncul).
// roles = role yang boleh diberikan; dioper handler karena mode single tak
// mengenal `owner` (0006 §7) dan view tak boleh menanyakan mode aplikasi.
func UsersPage(rows []UserRow, roles []string, canManageSuper bool) g.Node {
	return h.Div(
		h.H1(h.Class("text-xl font-semibold mb-4"), g.Text("Users")),
		// Slot toast (kanan-bawah), diisi via SSE patch (id "flash").
		// pointer-events:none agar toast tak memblokir klik elemen di bawahnya.
		h.Div(h.ID("flash"), h.Class("fixed bottom-4 right-4 z-50"),
			g.Attr("style", "pointer-events:none")),
		h.Div(
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Div(
				h.Class("card-body min-w-0"),
				ui.TableScroll(h.Table(
					h.Class("w-full text-sm"),
					h.THead(
						h.Tr(
							h.Class("border-b border-base-300 text-left text-base-content/70"),
							th("User"), th("Role"), th("Kuota"), th("Status"), th("Aksi"),
						),
					),
					h.TBody(g.Map(rows, func(u UserRow) g.Node { return UserRowNode(u, roles, canManageSuper) })),
				)),
			),
		),
	)
}

// UserRowNode merender satu baris user. Di-export agar handler bisa me-render
// ulang baris ini via SSE setelah mutasi (id "user-<id>" jadi target morph).
func UserRowNode(u UserRow, roles []string, canManageSuper bool) g.Node {
	rowID := "user-" + strconv.FormatInt(u.ID, 10)
	return h.Tr(
		h.ID(rowID),
		h.Class("border-b border-base-300"),
		// Kolom user: avatar + nama/email (+ badge root).
		h.Td(
			h.Class("py-2 pr-4"),
			h.Div(
				h.Class("flex items-center gap-2 min-w-0"),
				ui.Avatar(u.AvatarURL, u.Name, u.Email, 32),
				userIdent(u),
				ui.When(u.IsRoot, badge("root", "outline")),
			),
		),
		h.Td(h.Class("py-2 pr-4"), roleControl(u, roles, canManageSuper)),
		h.Td(h.Class("py-2 pr-4"), quotaControl(u)),
		h.Td(h.Class("py-2 pr-4"), statusControl(u)),
		h.Td(h.Class("py-2"), deleteControl(u)),
	)
}

// userIdent = nama di atas, email di bawahnya. Berbeda dari panel workspace,
// email di sini SELALU utuh: yang membacanya adalah operator platform, yang
// justru bertugas mencocokkan orang — dan nama tak bisa dipakai untuk itu
// (tidak unik, dan nilainya dikendalikan usernya sendiri di Google).
//
// Tanpa nama, email naik jadi baris utama alih-alih menyisakan baris kosong.
func userIdent(u UserRow) g.Node {
	if u.Name == "" {
		return h.Span(h.Class("truncate"), g.Text(u.Email))
	}
	return h.Div(
		h.Class("min-w-0"),
		h.Div(h.Class("truncate"), g.Text(u.Name)),
		h.Div(h.Class("truncate text-xs text-base-content/60"), g.Text(u.Email)),
	)
}

// quotaControl = jatah workspace user + tombol memberi/mencabut hak khusus.
//
// Menampilkan ASAL angkanya, bukan cuma angkanya: "3 global" vs "5 khusus".
// Tanpa penanda itu, operator tak bisa tahu siapa yang akan ikut berubah saat
// default global diubah — justru pertanyaan yang membuat halaman ini berguna.
// Root env dikecualikan: ia super_admin di semua workspace, kuota tak berlaku.
func quotaControl(u UserRow) g.Node {
	if u.IsRoot {
		return h.Span(h.Class("text-base-content/50 text-sm"), g.Text("—"))
	}
	id := strconv.FormatInt(u.ID, 10)
	origin := "global"
	if u.QuotaOverride {
		origin = "khusus"
	}
	nodes := []g.Node{
		h.Method("post"), h.Action("/dev/users/" + id + "/quota"),
		h.Class("flex flex-wrap items-center gap-1 min-w-0"),
		h.Input(
			h.Type("number"), h.Name("quota"),
			h.Value(strconv.Itoa(u.Quota)),
			g.Attr("min", "1"), g.Attr("max", "100"),
			h.Class("input input-sm w-16"),
		),
		h.Button(h.Type("submit"), h.Class("btn btn-sm"), g.Text("Set")),
		h.Span(h.Class("text-xs text-base-content/60"), g.Text(origin)),
	}
	form := h.FormEl(nodes...)
	if !u.QuotaOverride {
		return h.Div(h.Class("flex flex-col gap-1 min-w-0"), form)
	}
	// Hanya yang PUNYA hak khusus yang bisa dikembalikan ke global — tombol pada
	// yang sudah mengikuti global tak melakukan apa-apa selain membingungkan.
	return h.Div(
		h.Class("flex flex-col gap-1 min-w-0"),
		form,
		h.FormEl(
			h.Method("post"), h.Action("/dev/users/"+id+"/quota/reset"),
			h.Button(h.Type("submit"), h.Class("btn btn-xs btn-ghost"),
				g.Text("kembalikan ke global")),
		),
	)
}

// roleControl merender SATU baris kontrol per WORKSPACE tempat user jadi anggota
// (role kini per-workspace, bukan properti user). Root env → badge (immutable).
// Tanpa keanggotaan → penanda "—" (user ada, tapi belum/tak lagi di workspace mana pun).
func roleControl(u UserRow, roles []string, canManageSuper bool) g.Node {
	if u.IsRoot {
		return badge("root (semua workspace)", "")
	}
	if len(u.Workspaces) == 0 {
		return h.Span(h.Class("text-base-content/50 text-sm"), g.Text("—"))
	}
	items := make([]g.Node, 0, len(u.Workspaces))
	for _, ws := range u.Workspaces {
		items = append(items, h.Div(
			h.Class("flex items-center gap-2"),
			h.Span(h.Class("text-xs text-base-content/70 truncate max-w-[10rem]"), g.Text(ws.Name)),
			workspaceRoleSelect(u.ID, roles, ws),
		))
	}
	return h.Div(h.Class("flex flex-col gap-1"), g.Group(items))
}

// workspaceRoleSelect = dropdown ubah role user DI SATU workspace. tenant dikirim
// sebagai hidden field (handler butuh tahu workspace mana). Balasan SSE me-render
// ulang baris + toast (tanpa reload).
func workspaceRoleSelect(userID int64, roles []string, ws WorkspaceRole) g.Node {
	opts := make([]g.Node, 0, len(roles))
	for _, r := range roles {
		opts = append(opts, roleOption(r, ws.Role))
	}
	return ui.FormPostSelectWith(
		"/dev/users/"+strconv.FormatInt(userID, 10)+"/role", "role",
		map[string]string{"tenant": strconv.FormatInt(ws.TenantID, 10)},
		g.Group(opts),
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
	return ui.FormPostSelect("/dev/users/"+strconv.FormatInt(u.ID, 10)+"/status", "status",
		statusOption("active", u.Status),
		statusOption("disabled", u.Status),
		statusOption("blocked", u.Status),
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
		h.Class("btn btn-error btn-sm"),
		data.On("click", ui.PostAction("/dev/users/"+strconv.FormatInt(u.ID, 10)+"/delete")),
		g.Text("Hapus"),
	)
}

// Flash merender toast notifikasi (id "flash", target SSE patch). Auto-hilang
// via animasi CSS (.toast-flash, fade-out) — TANPA inline script (patuh CSP
// script-src 'self'). ok=true → hijau (alert-success), false → merah (alert-error).
// Catatan: class fade kustom bernama .toast-flash (BUKAN .toast) — daisyUI punya
// .toast sendiri (kontainer posisi) yang akan bentrok.
func Flash(ok bool, msg string) g.Node {
	cls := "alert alert-success toast-flash shadow-lg"
	if !ok {
		cls = "alert alert-error toast-flash shadow-lg"
	}
	return h.Div(
		h.ID("flash"),
		h.Class("fixed bottom-4 right-4 z-50"),
		g.Attr("style", "pointer-events:none"), // toast tak boleh blokir klik
		h.Div(
			h.Class(cls),
			h.Role("status"),
			g.Text(msg),
		),
	)
}

func th(label string) g.Node {
	return h.Th(h.Class("py-2 pr-4 font-medium"), g.Text(label))
}

// badge merender lencana daisyUI. variant "outline" → badge-outline; selain itu
// (kosong) → badge-neutral (abu solid) untuk role/status immutable root.
func badge(text, variant string) g.Node {
	cls := "badge badge-neutral"
	if variant == "outline" {
		cls = "badge badge-outline"
	}
	return h.Span(h.Class(cls), g.Text(text))
}
