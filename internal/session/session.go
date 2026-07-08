package session

import (
	"context"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/redis/rueidis"
)

// keyUserID adalah satu-satunya tempat string key ini ada — typo jadi mustahil (§4.9).
const keyUserID = "userID"

// mgr di-inject saat startup via Init. Accessor di bawah membungkusnya agar
// handler/middleware tidak pernah menyentuh string key langsung.
var mgr *scs.SessionManager

// NewManager membuat SessionManager dengan store rueidis.
func NewManager(client rueidis.Client) *scs.SessionManager {
	sm := scs.New()
	sm.Store = NewRueidisStore(client)
	sm.Lifetime = 24 * time.Hour
	sm.Cookie.HttpOnly = true
	sm.Cookie.SameSite = 3 // http.SameSiteStrictMode
	return sm
}

// Init menyimpan manager global agar accessor typed bisa dipakai.
func Init(sm *scs.SessionManager) { mgr = sm }

// UserID mengembalikan id user login, atau 0 bila belum login.
func UserID(ctx context.Context) int64 { return mgr.GetInt64(ctx, keyUserID) }

// SetUserID menandai user login (dipanggil setelah login sukses).
func SetUserID(ctx context.Context, id int64) { mgr.Put(ctx, keyUserID, id) }

// Clear menghancurkan session (logout). RenewToken juga dipakai saat login
// untuk mencegah session fixation.
func Clear(ctx context.Context) error { return mgr.Destroy(ctx) }

// Renew memutar token session (panggil setelah login sukses — anti fixation).
func Renew(ctx context.Context) error { return mgr.RenewToken(ctx) }

// WriteCookie meng-commit session ke store dan menulis Set-Cookie ke response
// SECARA MANUAL. WAJIB dipanggil SEBELUM membuka stream Datastar (NewSSE),
// karena NewSSE langsung flush header via http.ResponseController yang meng-Unwrap
// pembungkus scs — sehingga cookie dari LoadAndSave tidak akan pernah terkirim.
// Lihat catatan bug di STATER §4.6.
func WriteCookie(ctx context.Context, w http.ResponseWriter) error {
	token, expiry, err := mgr.Commit(ctx)
	if err != nil {
		return err
	}
	mgr.WriteSessionCookie(ctx, w, token, expiry)
	return nil
}
