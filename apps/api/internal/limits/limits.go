// Package limits builds the per-address, per-email and per-account rate
// limits Registre's public routes sit behind, over tronc's Ratelimit.
//
// tronc's limiter never evicts a key, which is fine for addresses behind
// Traefik and not fine for keys an attacker chooses, such as an email typed
// into a form. The email and account keys are therefore folded into 4096
// buckets by a digest: memory is bounded whatever arrives, and two people who
// share a bucket share a budget, which errs on the strict side.
package limits

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	troncmiddleware "github.com/FacileStudio/tronc/middleware"
)

// peekLimit is how much of a body the email key reads. A sign-up body is far
// smaller; a larger one keys on the empty address and is refused later.
const peekLimit = 16 << 10

// PerAddress limits each source address.
func PerAddress(limit float64, burst int) func(http.Handler) http.Handler {
	return troncmiddleware.Ratelimit(troncmiddleware.RatelimitConfig{Limit: limit, Burst: burst})
}

// PerEmail limits each email address named in a JSON body's "email" field.
func PerEmail(limit float64, burst int) func(http.Handler) http.Handler {
	return troncmiddleware.Ratelimit(troncmiddleware.RatelimitConfig{Limit: limit, Burst: burst, KeyFunc: emailKey})
}

// PerCookie limits each holder of the named cookie, so one account's
// session cannot spend a shared address's budget for everyone behind it.
func PerCookie(name string, limit float64, burst int) func(http.Handler) http.Handler {
	return troncmiddleware.Ratelimit(troncmiddleware.RatelimitConfig{
		Limit: limit, Burst: burst,
		KeyFunc: func(r *http.Request) string {
			cookie, err := r.Cookie(name)
			if err != nil {
				return "cookie:none"
			}
			return "cookie:" + bucket(cookie.Value)
		},
	})
}

// emailKey reads the body's email and puts the body back for the handler.
func emailKey(r *http.Request) string {
	raw, err := io.ReadAll(io.LimitReader(r.Body, peekLimit))
	if err != nil {
		return "email:unreadable"
	}
	r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(raw), r.Body))
	var body struct {
		Email string `json:"email"`
	}
	_ = json.Unmarshal(raw, &body)
	return "email:" + bucket(strings.ToLower(strings.TrimSpace(body.Email)))
}

// bucket folds a key into one of 4096 buckets.
func bucket(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:2])[:3]
}
