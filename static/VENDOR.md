# Aset Vendored

Prinsip "no CDN" (§13): tiap aset di-download lokal + checksum tercatat.
Upgrade = ganti versi di `Makefile` + sha256 di bawah, jalankan ulang `make setup` + `make css`.

| Aset          | Versi   | SHA-256 | Sumber |
|---------------|---------|---------|--------|
| datastar.js   | 1.0.2   | `2837d87acf6ee0ba8e4e63765926c25a98d63883b02f88be194a86b81d3fd24a` | https://cdn.jsdelivr.net/gh/starfederation/datastar@1.0.2/bundles/datastar.js |
| basecoat.css  | 1.0.2   | `8123677adb9bba43be3298e1543bcc5fc763e8cda3d32dc74c806046a3537ca0` | https://cdn.jsdelivr.net/npm/basecoat-css@1.0.2/dist/basecoat.cdn.min.css |
| tailwindcss (CLI) | v4.3.2 | — (binary, gitignored; di-download `make setup`) | https://github.com/tailwindlabs/tailwindcss/releases/tag/v4.3.2 |

> **Catatan versi:**
> - JS runtime Datastar (1.0.2) di-version TERPISAH dari Go SDK (`starfederation/datastar-go` v1.2.2). Kompatibel di jalur API v1. Jangan samakan nomornya.
> - `basecoat.cdn.min.css` = bundle self-contained (sudah termasuk Tailwind base + komponen). `app.css` (hasil `make css`) berisi utility layout dari scan file `.go`. Dua file terpisah, keduanya di-embed.
> - `app.css` **tidak** dicatat di sini (hasil generate, bukan vendored) — di-cache-bust dengan content-hash saat disajikan.
