package assets

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestInsertHash(t *testing.T) {
	cases := []struct{ name, hash, want string }{
		{"app.css", "abcd1234", "app.abcd1234.css"},
		{"app.min.css", "h", "app.min.h.css"}, // LastIndex titik
		{"noext", "h", "noext.h"},             // tanpa ekstensi
	}
	for _, c := range cases {
		if got := insertHash(c.name, c.hash); got != c.want {
			t.Errorf("insertHash(%q,%q)=%q, want %q", c.name, c.hash, got, c.want)
		}
	}
}

func TestNewAndPath(t *testing.T) {
	fsys := fstest.MapFS{"app.css": {Data: []byte("body{}")}}
	s, err := New(fsys, "app.css")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Path bustable → ber-hash.
	p := s.Path("app.css")
	if len(p) != len("/static/app.")+8+len(".css") || p[:12] != "/static/app." {
		t.Errorf("Path ber-hash salah bentuk: %q", p)
	}
	// Path tak terdaftar → passthrough.
	if got := s.Path("other.css"); got != "/static/other.css" {
		t.Errorf("passthrough salah: %q", got)
	}
}

func TestNew_MissingFile(t *testing.T) {
	if _, err := New(fstest.MapFS{}, "nope.css"); err == nil {
		t.Error("file bustable tak ada harus error")
	}
}

func TestHashFile_Deterministic(t *testing.T) {
	fsys := fstest.MapFS{
		"a": {Data: []byte("hello")},
		"b": {Data: []byte("hello")},
		"c": {Data: []byte("world")},
	}
	ha, _ := hashFile(fsys, "a")
	hb, _ := hashFile(fsys, "b")
	hc, _ := hashFile(fsys, "c")
	if len(ha) != 8 {
		t.Errorf("hash harus 8 char, got %d", len(ha))
	}
	if ha != hb {
		t.Error("konten sama harus hash sama")
	}
	if ha == hc {
		t.Error("konten beda harus hash beda")
	}
}

func TestHandler_HashedImmutable(t *testing.T) {
	fsys := fstest.MapFS{"app.css": {Data: []byte("body{color:red}")}}
	s, _ := New(fsys, "app.css")
	hashed := s.Path("app.css") // /static/app.<hash>.css

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, hashed, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("aset ber-hash harus 200, got %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control immutable salah: %q", cc)
	}
	if rec.Body.String() != "body{color:red}" {
		t.Errorf("isi file salah: %q", rec.Body.String())
	}
}

func TestHandler_UnhashedNoImmutable(t *testing.T) {
	fsys := fstest.MapFS{"app.css": {Data: []byte("x")}}
	s, _ := New(fsys, "app.css")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/app.css", nil))

	if rec.Header().Get("Cache-Control") == "public, max-age=31536000, immutable" {
		t.Error("nama asli (tanpa hash) tak boleh dapat header immutable")
	}
}

func TestHandler_NotFound(t *testing.T) {
	fsys := fstest.MapFS{"app.css": {Data: []byte("x")}}
	s, _ := New(fsys, "app.css")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/missing.css", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("file tak ada harus 404, got %d", rec.Code)
	}
}
