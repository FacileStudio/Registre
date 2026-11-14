package waitlist

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// ErrToken is an unsubscribe token that is not shaped like one.
var ErrToken = errors.New("waitlist: malformed unsubscribe token")

// tokenLabel separates the unsubscribe key from every other use of the key
// it is derived from.
const tokenLabel = "registre waitlist unsubscribe v1"

// maxTokenLength caps a token before anything decodes it.
const maxTokenLength = 256

// unsubscribeKey derives the key unsubscribe tokens are signed under.
func unsubscribeKey(root []byte) []byte {
	mac := hmac.New(sha256.New, root)
	mac.Write([]byte(tokenLabel))
	return mac.Sum(nil)
}

// Token is the unsubscribe token of one entry: its id and a MAC over it.
func (s *Service) Token(id string) string {
	return id + "." + base64.RawURLEncoding.EncodeToString(s.mac(id))
}

// Unsubscribe deletes the entry a token names. A token whose MAC does not
// match is malformed; a good token for an entry already gone succeeds, so a
// second click on the same link answers like the first.
func (s *Service) Unsubscribe(ctx context.Context, token string) error {
	id, ok := s.verify(token)
	if !ok {
		return ErrToken
	}
	err := s.db.WithContext(ctx).Where("id = ?", id).Delete(&schemas.WaitlistEntry{}).Error
	if err != nil {
		return fmt.Errorf("waitlist: unsubscribe: %w", err)
	}
	return nil
}

// verify returns the entry id a token carries when its MAC is genuine.
func (s *Service) verify(token string) (string, bool) {
	if len(token) > maxTokenLength {
		return "", false
	}
	id, sig, found := strings.Cut(token, ".")
	if !found || id == "" {
		return "", false
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, s.mac(id)) {
		return "", false
	}
	return id, true
}

// mac signs an entry id under the unsubscribe key.
func (s *Service) mac(id string) []byte {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(id))
	return mac.Sum(nil)
}
