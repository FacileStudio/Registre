// Package limits builds the per-address, per-session and per-key rate limits
// Registre's public routes sit behind, over tronc's Ratelimit.
//
// tronc's limiter never evicts a key, which is fine for addresses behind
// Traefik and not fine for keys a caller chooses, such as a cookie value or
// a space id. Those keys are folded into 4096 buckets by a digest: memory is
// bounded whatever arrives, and two callers who share a bucket share a
// budget, which errs on the strict side. An IPv6 address is reduced to its
// /64, because one host usually holds a whole /64 and would otherwise get a
// fresh budget per address.
package limits

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"

	troncmiddleware "github.com/FacileStudio/tronc/middleware"
)

// PerAddress limits each source address, IPv6 counted per /64.
func PerAddress(limit float64, burst int) func(http.Handler) http.Handler {
	return troncmiddleware.Ratelimit(troncmiddleware.RatelimitConfig{Limit: limit, Burst: burst, KeyFunc: AddressKey})
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

// AddressKey is the caller's address, an IPv6 one reduced to its /64. httpx's
// RealIP has already rewritten RemoteAddr from the trusted proxy headers.
func AddressKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() != nil {
		return "addr:" + host
	}
	return "addr6:" + ip.Mask(net.CIDRMask(64, 128)).String()
}

// Bucket folds a caller-chosen key into one of 4096 buckets.
func Bucket(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:2])[:3]
}
