// Package assets mengurus penyajian aset statis ber-cache-bust dari embed.FS.
package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"strings"
)

// Server menyajikan file dari embed.FS dengan cache-busting: file yang
// di-request lewat path ber-hash (mis. /static/app.a1b2c3d4.css) dilayani
// dengan Cache-Control immutable. Path tanpa hash tetap dilayani normal.
type Server struct {
	fsys   fs.FS
	hashed map[string]string // nama asli -> nama ber-hash (app.css -> app.<hash>.css)
	byHash map[string]string // nama ber-hash -> nama asli
	filerv http.Handler
}

// New membangun Server, menghitung hash untuk file yang di-cache-bust.
// bustable = daftar nama file (relatif ke root fsys) yang akan diberi hash.
func New(fsys fs.FS, bustable ...string) (*Server, error) {
	s := &Server{
		fsys:   fsys,
		hashed: make(map[string]string),
		byHash: make(map[string]string),
		filerv: http.FileServerFS(fsys),
	}
	for _, name := range bustable {
		h, err := hashFile(fsys, name)
		if err != nil {
			return nil, err
		}
		hashedName := insertHash(name, h)
		s.hashed[name] = hashedName
		s.byHash[hashedName] = name
	}
	return s, nil
}

// Path mengembalikan URL publik untuk aset (ber-hash bila terdaftar).
// mis. Path("app.css") -> "/static/app.a1b2c3d4.css".
func (s *Server) Path(name string) string {
	if hashed, ok := s.hashed[name]; ok {
		return "/static/" + hashed
	}
	return "/static/" + name
}

// Handler menyajikan aset di bawah /static/. File ber-hash mendapat
// Cache-Control immutable; sisanya no-cache pendek.
func (s *Server) Handler() http.Handler {
	return http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if orig, ok := s.byHash[name]; ok {
			// Aset ber-hash: konten immutable, cache setahun.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			r2 := new(http.Request)
			*r2 = *r
			r2.URL.Path = "/" + orig
			s.filerv.ServeHTTP(w, r2)
			return
		}
		s.filerv.ServeHTTP(w, r)
	}))
}

func hashFile(fsys fs.FS, name string) (string, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hsh := sha256.New()
	if _, err := io.Copy(hsh, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hsh.Sum(nil))[:8], nil
}

// insertHash menyisipkan hash sebelum ekstensi: app.css -> app.<hash>.css.
func insertHash(name, hash string) string {
	dot := strings.LastIndex(name, ".")
	if dot < 0 {
		return name + "." + hash
	}
	return name[:dot] + "." + hash + name[dot:]
}
