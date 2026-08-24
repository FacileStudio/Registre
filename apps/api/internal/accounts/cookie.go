package accounts

import (
	"net/http"
)

// Cookies writes and reads the account cookie for one deployment. The name
// depends on the issuer's scheme, so it is resolved once at boot rather than
// re-decided at every call site.
type Cookies struct {
	name   string
	secure bool
}

// NewCookies returns the cookie policy for an issuer. A secure deployment
// gets the __Host- prefixed name, which the browser refuses unless the cookie
// really is Secure, path "/" and domainless, so the guarantee is enforced by
// the browser and not by this code remembering to.
func NewCookies(secure bool) Cookies {
	if secure {
		return Cookies{name: CookieName, secure: true}
	}
	return Cookies{name: DevCookieName}
}

// Name is the cookie this deployment sets.
func (c Cookies) Name() string { return c.name }

// Write sets the account cookie to token.
//
// SameSite is Lax rather than Strict on purpose: the account pages are
// reached by following a redirect back from the OIDC flow, and Strict would
// withhold the cookie on exactly that navigation, so the human would arrive
// logged out of the page they just logged in to. Lax withholds it from
// cross-site POSTs, which is the case that matters, and CSRFHeader covers the
// rest.
func (c Cookies) Write(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     c.name,
		Value:    token,
		Path:     "/",
		MaxAge:   int(Lifetime.Seconds()),
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// Clear expires the account cookie.
func (c Cookies) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     c.name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// Read returns the account cookie's value, or the empty string.
func (c Cookies) Read(r *http.Request) string {
	cookie, err := r.Cookie(c.name)
	if err != nil {
		return ""
	}
	return cookie.Value
}
