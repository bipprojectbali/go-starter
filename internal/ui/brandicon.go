package ui

import (
	g "maragu.dev/gomponents"
)

// GoogleG merender logo "super G" 4-warna RESMI Google.
//
// KEPATUHAN MEREK (wajib — syarat verifikasi OAuth app, lihat
// developers.google.com/identity/branding-guidelines):
//   - Warna 4 path TIDAK BOLEH diubah: #EA4335 #4285F4 #FBBC05 #34A853.
//   - Aspect ratio tetap (viewBox 0 0 48 48). Ukuran diatur via class
//     (mis. "size-[18px]"), bukan mengubah viewBox.
//   - Logo harus di dalam tombol BERSAMA teks — bukan "G" telanjang.
//
// Dibuat dengan primitif g.El/g.Attr (paket svg tak ada di gomponents v1.3.0).
func GoogleG(attrs ...g.Node) g.Node {
	base := []g.Node{
		g.Attr("viewBox", "0 0 48 48"),
		g.Attr("aria-hidden", "true"),
		g.Attr("focusable", "false"),
	}
	return g.El("svg", append(append(base, attrs...),
		path("#EA4335", "M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"),
		path("#4285F4", "M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"),
		path("#FBBC05", "M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"),
		path("#34A853", "M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"),
	)...)
}

// path adalah helper <path fill=… d=…> untuk logo brand.
func path(fill, d string) g.Node {
	return g.El("path", g.Attr("fill", fill), g.Attr("d", d))
}
