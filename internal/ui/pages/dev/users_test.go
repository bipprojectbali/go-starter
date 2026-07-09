package dev

import (
	"strings"
	"testing"
)

// TestUserRow_SelectWrappedInForm menjaga agar select role/status dibungkus
// <form>. @post {contentType:'form'} mencari form TERDEKAT untuk kirim value —
// tanpa form, tak ada value terkirim (perubahan role gagal senyap).
func TestUserRow_SelectWrappedInForm(t *testing.T) {
	row := UserRow{ID: 7, Email: "u@x.com", Role: "user", Status: "active"}
	var sb strings.Builder
	UserRowNode(row, true).Render(&sb)
	out := sb.String()

	if !strings.Contains(out, `<form><select class="select select-sm" name="role"`) {
		t.Errorf("select role harus dibungkus <form> (contentType:form butuh form):\n%s", out)
	}
	if !strings.Contains(out, `<form><select class="select select-sm" name="status"`) {
		t.Errorf("select status harus dibungkus <form>:\n%s", out)
	}
	// @post ke endpoint yang benar (kutip di-HTML-escape jadi &#39;).
	if !strings.Contains(out, "/dev/users/7/role") {
		t.Errorf("select role harus @post ke /dev/users/7/role:\n%s", out)
	}
}

// TestUserRow_RootImmutable: root env → badge (bukan dropdown), tanpa tombol hapus.
func TestUserRow_RootImmutable(t *testing.T) {
	row := UserRow{ID: 1, Email: "root@x.com", Role: "super_admin", Status: "active", IsRoot: true}
	var sb strings.Builder
	UserRowNode(row, true).Render(&sb)
	out := sb.String()

	if strings.Contains(out, "/dev/users/1/role") {
		t.Errorf("root tak boleh punya kontrol ubah role:\n%s", out)
	}
	if strings.Contains(out, "/dev/users/1/delete") {
		t.Errorf("root tak boleh punya tombol hapus:\n%s", out)
	}
	if !strings.Contains(out, `class="badge badge-neutral"`) {
		t.Errorf("root harus tampil sbg badge:\n%s", out)
	}
}

// TestFlash: toast punya class .toast-flash (auto-dismiss CSS) & pesan.
func TestFlash(t *testing.T) {
	var ok, bad strings.Builder
	Flash(true, "Berhasil").Render(&ok)
	Flash(false, "Gagal").Render(&bad)

	if !strings.Contains(ok.String(), "toast-flash") || !strings.Contains(ok.String(), "Berhasil") {
		t.Errorf("flash sukses kurang toast-flash/pesan:\n%s", ok.String())
	}
	if !strings.Contains(ok.String(), "alert-success") {
		t.Errorf("flash sukses harus alert-success:\n%s", ok.String())
	}
	if !strings.Contains(bad.String(), "alert-error") {
		t.Errorf("flash gagal harus alert-error:\n%s", bad.String())
	}
}
