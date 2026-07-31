#!/usr/bin/env bash
# rename.sh — ganti nama project setelah meng-clone template ini.
#
# Dipanggil lewat `make rename name=nama-baru`. Yang diubah:
#   1. module path di go.mod + SELURUH import "go_starter/..." di file .go
#   2. nama database di .env.example
#   3. baris judul di main.go & README
#
# Yang TIDAK diubah, sengaja:
#   - .env milik Anda (berisi kredensial nyata; menyuntingnya berisiko dan
#     Anda memang perlu meninjau isinya sendiri)
#   - riwayat git & CHANGELOG (catatan sejarah template, bukan milik project baru)
#   - brand di layar — itu datang dari APP_NAME di .env, bukan dari kode
#
# Idempotent: dijalankan dua kali pada nama yang sama tak mengubah apa pun.

set -euo pipefail

NEW="${1:-}"
if [[ -z "$NEW" ]]; then
    echo "ERROR: nama project kosong. Pakai: make rename name=nama-project" >&2
    exit 2
fi

# Module path Go tak boleh memuat spasi atau huruf kapital yang menyulitkan;
# batasi ke bentuk yang pasti sah supaya kegagalannya muncul DI SINI, bukan
# sebagai error kompilasi yang membingungkan setelah ratusan file tersentuh.
if [[ ! "$NEW" =~ ^[a-z][a-z0-9_-]*$ ]]; then
    echo "ERROR: nama '$NEW' tak sah — pakai huruf kecil, angka, - atau _ (diawali huruf)." >&2
    exit 2
fi

OLD="$(head -1 go.mod | awk '{print $2}')"
if [[ "$OLD" == "$NEW" ]]; then
    echo "Nama project sudah '$NEW' — tak ada yang diubah."
    exit 0
fi

# Bekerja di repo yang BERSIH. Rename menyentuh ratusan file sekaligus; kalau
# ada perubahan lain yang belum di-commit, keduanya bercampur dan `git diff`
# tak lagi bisa dipakai meninjau apa yang dilakukan skrip ini.
if [[ -n "$(git status --porcelain 2>/dev/null)" ]]; then
    echo "ERROR: ada perubahan belum di-commit. Commit atau stash dulu," >&2
    echo "       supaya hasil rename bisa ditinjau sebagai satu diff utuh." >&2
    exit 1
fi

echo "Mengganti nama project: $OLD → $NEW"

# 1. Module path + seluruh import. -type f + -name '*.go' saja: file lain yang
#    menyebut nama project (CHANGELOG, docs) adalah catatan sejarah.
sed -i.bak "s|^module $OLD$|module $NEW|" go.mod && rm -f go.mod.bak
find . -name '*.go' -not -path './tmp/*' -print0 |
    xargs -0 sed -i.bak "s|\"$OLD/|\"$NEW/|g"
find . -name '*.go.bak' -delete

# 2. Nama database contoh. .env milik user TIDAK disentuh (lihat catatan di atas).
if [[ -f .env.example ]]; then
    sed -i.bak "s|/${OLD}|/${NEW}|g; s|^# ${OLD} |# ${NEW} |" .env.example && rm -f .env.example.bak
fi

# 3. Baris judul (komentar package & judul README). Sengaja hanya baris PERTAMA:
#    penyebutan lain di dokumentasi menjelaskan asal-usul template, dan
#    mengganti semuanya membuat rujukan sejarah jadi salah.
sed -i.bak "1s|Command $OLD|Command $NEW|" main.go && rm -f main.go.bak
[[ -f README.md ]] && { sed -i.bak "1s|# $OLD|# $NEW|" README.md && rm -f README.md.bak; }

echo
echo "Selesai. Langkah berikutnya:"
echo "  1. Sesuaikan .env — DATABASE_URL, TEST_DATABASE_URL, dan APP_NAME"
echo "     (APP_NAME yang menentukan nama di sidebar & halaman depan)"
echo "  2. createdb ${NEW} && createdb ${NEW}_test"
echo "  3. make check    # buktikan semuanya masih hijau"
echo
echo "Tinjau hasilnya dengan: git diff --stat"
