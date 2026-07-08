// Package session mengurus session HTTP: store rueidis + accessor typed.
package session

import (
	"context"
	"time"

	"github.com/redis/rueidis"
)

// RueidisStore mengimplementasikan scs.Store di atas redis/rueidis.
// Alasan Store kustom (bukan redisstore resmi): redisstore menyeret klien
// redigo — Store 3-method ini menjaga SATU klien Redis (rueidis) di binary.
type RueidisStore struct {
	client rueidis.Client
	prefix string
}

// NewRueidisStore membuat store dengan prefix key "scs:session:".
func NewRueidisStore(client rueidis.Client) *RueidisStore {
	return &RueidisStore{client: client, prefix: "scs:session:"}
}

// Find mengembalikan data session untuk token. Token hilang/kedaluwarsa =
// (nil, false, nil) — err hanya untuk kegagalan sistem (kontrak scs.Store).
func (s *RueidisStore) Find(token string) ([]byte, bool, error) {
	ctx := context.Background()
	b, err := s.client.Do(ctx, s.client.B().Get().Key(s.prefix+token).Build()).AsBytes()
	if err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return b, true, nil
}

// Commit menyimpan data session dengan TTL diturunkan dari expiry.
func (s *RueidisStore) Commit(token string, b []byte, expiry time.Time) error {
	ctx := context.Background()
	ttl := int64(time.Until(expiry).Seconds())
	if ttl < 1 {
		ttl = 1 // guard TTL <1s (SET EX menolak 0)
	}
	return s.client.Do(ctx,
		s.client.B().Set().Key(s.prefix+token).Value(rueidis.BinaryString(b)).ExSeconds(ttl).Build(),
	).Error()
}

// Delete menghapus token. Token tidak ada = no-op (DEL mengembalikan 0, bukan error).
func (s *RueidisStore) Delete(token string) error {
	ctx := context.Background()
	return s.client.Do(ctx, s.client.B().Del().Key(s.prefix+token).Build()).Error()
}
