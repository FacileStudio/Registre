// Package limits builds the per-address, per-session and per-key rate limits
// Registre's public routes sit behind, over tronc's Ratelimit.
//
// Keys a caller chooses, such as a cookie value or a space id, are folded
// into 4096 buckets by a digest: memory is bounded whatever arrives, and two
// callers who share a bucket share a budget, which errs on the strict side.
// Addresses are keyed by tronc's RemoteAddrKey64, which reduces an IPv6
// address to its /64 because one host usually holds a whole /64 and would
// otherwise get a fresh budget per address, and the limiter remembers at most
// MaxAddresses of them, forgetting the longest idle first.
package limits

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	troncmiddleware "github.com/FacileStudio/tronc/middleware"
)

// MaxAddresses bounds the addresses one per-address limiter remembers, the
// bound tronc's rate limiting guide sets for every limiter facing the public
// internet.
const MaxAddresses = 50000

// PerAddress limits each source address, IPv6 counted per /64. httpx's RealIP
// has already rewritten RemoteAddr from the trusted proxy headers.
func PerAddress(limit float64, burst int) func(http.Handler) http.Handler {
	return troncmiddleware.Ratelimit(troncmiddleware.RatelimitConfig{
		Limit: limit, Burst: burst, KeyFunc: troncmiddleware.RemoteAddrKey64, MaxKeys: MaxAddresses,
	})
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
			return "cookie:" + Bucket(cookie.Value)
		},
	})
}

// Bucket folds a caller-chosen key into one of 4096 buckets.
func Bucket(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:2])[:3]
}
