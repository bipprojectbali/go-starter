# Aset Vendored

Prinsip "no CDN" (§13): tiap aset di-download lokal + checksum tercatat.
Upgrade = ganti versi di `Makefile` + sha256 di bawah, jalankan ulang `make setup` + `make css`.

| Aset          | Versi   | SHA-256 | Sumber |
|---------------|---------|---------|--------|
| datastar.js   | 1.0.2   | `2837d87acf6ee0ba8e4e63765926c25a98d63883b02f88be194a86b81d3fd24a` | https://cdn.jsdelivr.net/gh/starfederation/datastar@1.0.2/bundles/datastar.js |
| basecoat.css  | 1.0.2   | `8123677adb9bba43be3298e1543bcc5fc763e8cda3d32dc74c806046a3537ca0` | https://cdn.jsdelivr.net/npm/basecoat-css@1.0.2/dist/basecoat.cdn.min.css |
| mermaid.min.js | 11 (11.12.x) | `74d7c46dabca328c2294733910a8aa1ed0c37451776e8d5295da38a2b758fb9b` | https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js (UMD, dev-only ERD di /dev/erd) |
| tailwindcss (CLI) | v4.3.2 | `b800b0659dc64b9f03ede5660244d9415d777d5739ae2889280877ca37be742a` (macos-arm64, 79.759.970 B) | https://github.com/tailwindlabs/tailwindcss/releases/tag/v4.3.2 |

> **Catatan versi:**
> - JS runtime Datastar (1.0.2) di-version TERPISAH dari Go SDK (`starfederation/datastar-go` v1.2.2). Kompatibel di jalur API v1. Jangan samakan nomornya.
> - `basecoat.cdn.min.css` = bundle self-contained (sudah termasuk Tailwind base + komponen). `app.css` (hasil `make css`) berisi utility layout dari scan file `.go`. Dua file terpisah, keduanya di-embed.
> - `app.css` **tidak** dicatat di sini (hasil generate, bukan vendored) — di-cache-bust dengan content-hash saat disajikan.
> - Binary `tailwindcss` (gitignored) di-download `make setup`/`make tailwind` untuk OS/arch ini. **Wajib utuh**: unduh parsial (mis. 50 MB, bukan ~80 MB) menghasilkan Mach-O terpotong → di Apple Silicon di-SIGKILL kernel jadi *"Malformed Mach-o file"*. `make tailwind` kini exec-test hasil download & menolak file korup. SHA-256 di atas khusus macos-arm64; arch lain beda hash.
