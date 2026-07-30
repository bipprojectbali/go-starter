package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// WorkspaceView = data siap-render halaman pengaturan. Struct (bukan deretan
// parameter) karena jumlahnya sudah melewati batas nyaman, dan tiap flag punya
// arti berbeda yang mudah tertukar bila hanya berupa bool berurutan.
type WorkspaceView struct {
	Base     string // prefix URL workspace ("/w/acme") — dioper handler, bukan dirakit view
	Name     string
	Slug     string
	Status   string // active | suspended | archived
	CanEdit  bool   // owner/platform: ganti nama
	CanOwn   bool   // owner/platform: arsip & hapus (zona bahaya)
	Archived bool
	// ErrMsg = pesan dari redirect PRG (?err=CODE). Aksi siklus hidup memakai
	// native POST + 303, jadi tanpa slot ini penolakannya HILANG SENYAP: user
	// menekan tombol, halaman termuat ulang tampak normal, dan tak ada apa pun
	// yang menjelaskan kenapa tak terjadi apa-apa.
	ErrMsg string
}

// Workspace merender halaman pengaturan workspace: form ganti NAMA tampilan +
// zona bahaya (arsip/hapus). canEdit=false (mis. admin non-owner) → field
// disabled, hanya lihat. slug ditampilkan read-only (identitas immutable).
func Workspace(v WorkspaceView) g.Node {
	fields := []g.Node{
		h.Div(
			h.Class("grid gap-2"),
			ui.Label("Nama Workspace", h.For("name")),
			ui.Input(
				h.ID("name"),
				h.Type("text"),
				data.Bind("name"),
				h.Placeholder("mis. Acme Corp"),
				g.If(!v.CanEdit || v.Archived, h.Disabled()),
			),
		),
		h.Div(
			h.Class("grid gap-2"),
			ui.Label("Slug (identitas, tak bisa diubah)", h.For("slug")),
			ui.Input(h.ID("slug"), h.Type("text"), h.Value(v.Slug), h.Disabled()),
		),
	}
	switch {
	case v.Archived:
		fields = append(fields, h.P(
			h.Class("text-sm text-base-content/60"),
			g.Text("Workspace diarsipkan — hanya-baca. Aktifkan kembali untuk mengubah."),
		))
	case v.CanEdit:
		fields = append(fields,
			ui.Button(
				ui.VariantDefault,
				[]g.Node{data.On("click", ui.PostAction(v.Base+"/settings"))},
				g.Text("Simpan"),
			),
			ui.AlertSlot("workspace-msg"),
		)
	default:
		fields = append(fields, h.P(
			h.Class("text-sm text-base-content/60"),
			g.Text("Hanya owner yang dapat mengubah nama workspace."),
		))
	}

	body := []g.Node{
		data.Signals(map[string]any{"name": v.Name}),
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Pengaturan Workspace")),
		h.P(h.Class("text-base-content/70 mb-4"), g.Text("Kelola nama workspace Anda.")),
	}
	if v.ErrMsg != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "workspace-err", g.Text(v.ErrMsg)))
	}
	body = append(body, ui.Card(fields...))
	if v.CanOwn {
		body = append(body, dangerZone(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// dangerZone = arsip & hapus. Dipisah visual (border error) karena keduanya
// mengubah keadaan workspace bagi SEMUA anggota, bukan cuma bagi yang menekan.
// Hanya owner/platform yang melihatnya sama sekali.
func dangerZone(v WorkspaceView) g.Node {
	var actions []g.Node
	if v.Archived {
		// Pemulihan diletakkan di LUAR prefix /w/{slug} (0005 §4): kalau di dalam,
		// gerbang read-only-nya sendiri akan memblokir POST ini — pintu keluar
		// terkunci dari dalam.
		actions = append(actions,
			lifecycleAction(
				"/workspace/"+v.Slug+"/unarchive",
				"Aktifkan kembali",
				"Workspace kembali bisa diubah oleh anggotanya.",
				"btn-primary",
			),
		)
	} else {
		actions = append(actions,
			lifecycleAction(
				v.Base+"/archive",
				"Arsipkan",
				"Workspace jadi hanya-baca. Data tetap utuh dan bisa diaktifkan lagi kapan saja.",
				"btn-warning btn-outline",
			),
		)
	}
	actions = append(actions,
		lifecycleAction(
			v.Base+"/delete",
			"Hapus workspace",
			"Bisa dibatalkan dalam 30 hari. Setelah itu data dihapus permanen.",
			"btn-error btn-outline",
		),
	)
	return h.Div(
		h.Class("card bg-base-100 border border-error/40 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold text-error mb-1"), g.Text("Zona Bahaya")),
			h.P(h.Class("text-sm text-base-content/70 mb-3"),
				g.Text("Tindakan di bawah memengaruhi seluruh anggota workspace.")),
			h.Div(h.Class("grid gap-3"), g.Group(actions)),
		),
	)
}

// lifecycleAction = satu baris aksi: penjelasan + tombol form POST native.
// Native POST → 303 (gotcha #16: redirect via SSE menyuntik <script> yang
// diblokir CSP). Baris pakai flex-wrap agar tak mendorong halaman di 375px.
func lifecycleAction(action, label, desc, btnClass string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(action),
		h.Class("flex flex-wrap items-center justify-between gap-2 min-w-0"),
		h.P(h.Class("text-sm text-base-content/70 flex-1 min-w-0 break-words"), g.Text(desc)),
		h.Button(h.Type("submit"), h.Class("btn btn-sm "+btnClass), g.Text(label)),
	)
}

// WorkspaceSuspended = halaman 403 untuk anggota workspace yang ditangguhkan
// platform. SENGAJA menjelaskan (bukan 404 seperti slug asing): penerimanya
// sudah terbukti anggota, dan tanpa alasan ia akan mengira workspace-nya hilang
// lalu menghubungi support tanpa perlu (0005 §3).
func WorkspaceSuspended(name, reason string) g.Node {
	body := []g.Node{
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Workspace Ditangguhkan")),
		h.P(h.Class("text-base-content/70 mb-4"),
			g.Text("Akses ke workspace "+name+" sedang ditangguhkan oleh pengelola platform.")),
	}
	if reason != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "suspend-reason", g.Text(reason)))
	}
	body = append(body, h.P(
		h.Class("text-sm text-base-content/60 mt-4"),
		g.Text("Data Anda tetap utuh. Hubungi pengelola platform untuk mengaktifkannya kembali."),
	))
	return h.Div(h.Class("grid gap-4 min-w-0 max-w-xl mx-auto py-8"), g.Group(body))
}
