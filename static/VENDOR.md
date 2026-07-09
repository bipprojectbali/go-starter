# Aset Vendored

Prinsip "no CDN" (§13): tiap aset di-download lokal + checksum tercatat.
Upgrade = ganti versi di `Makefile` + sha256 di bawah, jalankan ulang `make setup` + `make css`.

| Aset          | Versi   | SHA-256 | Sumber |
|---------------|---------|---------|--------|
| datastar.js   | 1.0.2   | `2837d87acf6ee0ba8e4e63765926c25a98d63883b02f88be194a86b81d3fd24a` | https://cdn.jsdelivr.net/gh/starfederation/datastar@1.0.2/bundles/datastar.js |
| daisyui.js    | 5.6.16  | `40833e8c61733e0121cfd5f5abf61dd95c74ece478798a15f1cec48690465dbf` | https://cdn.jsdelivr.net/npm/daisyui@5.6.16/+esm (plugin Tailwind v4, di-`@plugin` dari input.css) |
| mermaid.min.js | 11 (11.12.x) | `74d7c46dabca328c2294733910a8aa1ed0c37451776e8d5295da38a2b758fb9b` | https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js (UMD, dev-only ERD di /dev/erd) |
| echarts.min.js | 6.1.0 | `b66b25aeb4df84e33199dc21694014d336d222cbd9deb0e5a7c14bd6aa0d0fd0` | https://cdn.jsdelivr.net/npm/echarts@6.1.0/dist/echarts.min.js (UMD, chart panel /dev/logs; init di `charts.js` — CSP-safe) |
| tailwindcss (CLI) | v4.3.2 | `b800b0659dc64b9f03ede5660244d9415d777d5739ae2889280877ca37be742a` (macos-arm64, 79.759.970 B) | https://github.com/tailwindlabs/tailwindcss/releases/tag/v4.3.2 |

> **Catatan versi:**
> - JS runtime Datastar (1.0.2) di-version TERPISAH dari Go SDK (`starfederation/datastar-go` v1.2.2). Kompatibel di jalur API v1. Jangan samakan nomornya.
> - `daisyui.js` = plugin Tailwind v4 (di-`@plugin "./daisyui.js"` dari `input.css`). Bukan file yang dimuat browser — dikonsumsi Tailwind CLI saat `make css` untuk menghasilkan class komponen (btn, card, modal, alert, badge, table, input, select) + token tema ke dalam `app.css`. Satu pipeline, satu preflight (menggantikan bundle `basecoat.css` terpisah). File ini WAJIB ada saat build (di-commit ke repo).
> - `app.css` **tidak** dicatat di sini (hasil generate, bukan vendored) — di-cache-bust dengan content-hash saat disajikan.
> - Binary `tailwindcss` (gitignored) di-download `make setup`/`make tailwind` untuk OS/arch ini. **Wajib utuh**: unduh parsial (mis. 50 MB, bukan ~80 MB) menghasilkan Mach-O terpotong → di Apple Silicon di-SIGKILL kernel jadi *"Malformed Mach-o file"*. `make tailwind` kini exec-test hasil download & menolak file korup. SHA-256 di atas khusus macos-arm64; arch lain beda hash.
