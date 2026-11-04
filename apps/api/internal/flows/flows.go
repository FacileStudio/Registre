// Package flows is the per-flow browser cookie that binds an auth request to
// the browser that started it (migration 00014, modules/oidc's Binding).
//
// /authorize sets a random cookie and stores its SHA-256 as
// auth_requests.browser_hash; anything that later acts on that auth request
// on a browser's behalf asks Holds whether this browser is the one. The
// callback asks it before handing out a code, and sign-up asks it before
// carrying a pending app login through the emailed link.
package flows

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
)

// cookiePrefix names the per-flow cookie. Each flow gets its own cookie,
// suffixed with part of its digest, so a browser opening three apps at once
// holds three flows rather than the last one overwriting the first two.
const cookiePrefix = "registre_flow_"

// CookieName is the flow cookie for digest. A secure issuer gets the __Host-
// prefix, so the browser refuses the cookie unless it is Secure, on path "/"
// and domainless.
func CookieName(digest string, secure bool) string {
	if len(digest) < 16 {
		return ""
	}
	name := cookiePrefix + digest[:16]
	if secure {
		return "__Host-" + name
	}
	return name
}

// Digest is the hex SHA-256 of a flow cookie value. The value is a 256-bit
// random, so a fast hash is enough to keep the table from holding the cookie.
func Digest(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// Holds reports whether r carries the flow cookie whose digest is
// browserHash, and answers its name so the caller can clear it. An empty
// browserHash, a request bound to no browser, never matches.
func Holds(r *http.Request, browserHash string, secure bool) (string, bool) {
	name := CookieName(browserHash, secure)
	if name == "" {
		return "", false
	}
	cookie, err := r.Cookie(name)
	if err != nil {
		return "", false
	}
	if subtle.ConstantTimeCompare([]byte(Digest(cookie.Value)), []byte(browserHash)) != 1 {
		return "", false
	}
	return name, true
}
