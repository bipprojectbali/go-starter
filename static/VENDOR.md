# Aset Vendored

Prinsip "no CDN" (§13): tiap aset di-download lokal + checksum tercatat.
Upgrade = ganti versi + sha256, jalankan `make vendor-verify`.

| Aset          | Versi (JS runtime) | SHA-256 | Sumber |
|---------------|--------------------|---------|--------|
| datastar.js   | 1.0.2              | `2837d87acf6ee0ba8e4e63765926c25a98d63883b02f88be194a86b81d3fd24a` | https://cdn.jsdelivr.net/gh/starfederation/datastar@1.0.2/bundles/datastar.js |

> **Catatan versi:** JS runtime Datastar (1.0.2) di-version TERPISAH dari Go SDK
> (`starfederation/datastar-go` v1.2.2). Keduanya kompatibel di jalur API v1
> (Patch* / signals). Jangan samakan nomornya.
