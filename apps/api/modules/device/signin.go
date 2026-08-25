package device

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
)

// authenticate sends a human with no session through registre's own front door
// and back to the verification page.
//
// It exists because the primary path was the broken one. The CLI prints "open
// this URL and type this code", and the device most likely to be holding the
// URL is a phone, which is the device least likely to already hold a registre
// session. Before this route the only way to get one was to go and sign in to
// some other Facile app first and come back, which is a workaround printed on a
// screen rather than a login.
//
// It is a plain 302 into /authorize, so nothing new authenticates anybody:
// /authorize → /login?auth_request= → login.complete is the path every app in
// the suite already uses, and the account cookie this needs is written at the
// end of it either way. The authorization code that comes back is deliberately
// thrown away — the verification page never reads it, and it is unredeemable
// anyway, because the verifier behind the PKCE challenge below is discarded the
// moment this function returns.
//
// # Nothing in the destination comes from the request
//
// Every part of the Location header is process configuration: the authorization
// endpoint is taken from the provider at boot, and the redirect URI is built
// from the configured issuer, which is static by construction because a
// provider that trusts the Host header signs tokens for whatever issuer an
// attacker asks it to. No query parameter, header or body value is read here at
// all — there is deliberately no `return_to` and no `next`, because the moment
// any part of the destination is caller-supplied this stops being a redirect
// and starts being an open redirector.
//
// The user code is not threaded through either, and that is the second reason
// this shape is safe rather than merely convenient. It stays in the tab's
// sessionStorage, which dies with the tab and never reaches an access log, a
// referrer or a history entry. A low-entropy code that survives a round trip
// through registre's own logs twice would be exactly what SPEC §7 says a
// credential in a URL costs.
//
// The register is the second layer under all of that: RedirectURIs are
// byte-compared at /authorize, so even a destination that did come from the
// request would have to already be registered to go anywhere.
func (s *Service) authenticate(w http.ResponseWriter, r *http.Request) {
	challenge, err := pkceChallenge()
	if err != nil {
		s.refuse(w, err)
		return
	}
	query := url.Values{
		"client_id":             {clients.SelfClientID},
		"redirect_uri":          {s.redirectURI},
		"response_type":         {string(zoidc.ResponseTypeCode)},
		"scope":                 {string(zoidc.ScopeOpenID)},
		"code_challenge":        {challenge},
		"code_challenge_method": {string(zoidc.CodeChallengeMethodS256)},
	}
	http.Redirect(w, r, s.authorizeURL+"?"+query.Encode(), http.StatusFound)
}

// pkceChallenge mints a challenge whose verifier nobody keeps.
//
// That is the point rather than an oversight. registre is a public client, so
// zitadel's token endpoint refuses it a code exchange without PKCE — and with
// PKCE it refuses one without the verifier. Discarding the verifier here means
// the authorization code this flow leaves in somebody's address bar cannot be
// exchanged by anyone, including registre. The flow is being used for the
// session login opens along the way, not for the token at the end of it, and
// the code should be inert to match.
//
// There is no state parameter for the same reason: state protects a callback
// handler against a forged code, and there is no callback handler. The page the
// browser lands on reads the user code out of its own storage and ignores every
// query parameter but one it puts there itself.
func pkceChallenge() (string, error) {
	verifier := make([]byte, 32)
	if _, err := rand.Read(verifier); err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(verifier)
	sum := sha256.Sum256([]byte(encoded))
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}
