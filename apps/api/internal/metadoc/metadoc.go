// Package metadoc validates OAuth client metadata: a client ID metadata
// document (CIMD) that registre fetches from the URL a client uses as its
// client_id, and the body of a dynamic client registration (RFC 7591).
//
// Both describe a public client that registre has never met, so both are
// held to the same rules: redirect URIs a native or web client may really
// use, the authorization code grant only, and no client secret, because a
// document anyone can read cannot carry one.
package metadoc

import (
	"errors"
	"net/url"
)

// MaxDocument is the largest metadata document registre reads. The CIMD
// draft suggests 5 KB; the margin is for long client names and many
// redirect URIs, not for anything a client could need more of.
const MaxDocument = 16 << 10

// ErrInvalid is metadata registre refuses. The wrapped message says why, in
// words a client developer can act on.
var ErrInvalid = errors.New("metadoc: invalid client metadata")

// ErrRedirect is the part of ErrInvalid that is about a redirect URI.
var ErrRedirect = errors.New("metadoc: invalid redirect URI")

// Metadata is the part of RFC 7591 client metadata registre reads. Anything
// else in a document is ignored.
type Metadata struct {
	ClientID                string   `json:"client_id,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
	ClientURI               string   `json:"client_uri,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	ApplicationType         string   `json:"application_type,omitempty"`
	ClientSecret            string   `json:"client_secret,omitempty"`
}

// Validate refuses metadata registre cannot register as a public client and
// returns it with defaults filled in: application_type native, the code and
// refresh grants, the code response type and auth method none.
func Validate(m Metadata) (Metadata, error) {
	if m.ClientSecret != "" {
		return Metadata{}, invalid("client_secret must not be present: the client is public")
	}
	switch m.TokenEndpointAuthMethod {
	case "", "none":
	default:
		return Metadata{}, invalid("token_endpoint_auth_method must be none")
	}
	if err := checkGrants(m); err != nil {
		return Metadata{}, err
	}
	if m.ApplicationType == "" {
		m.ApplicationType = "native"
	}
	if m.ApplicationType != "native" && m.ApplicationType != "web" {
		return Metadata{}, invalid("application_type must be native or web")
	}
	if err := checkRedirects(m.RedirectURIs, m.ApplicationType); err != nil {
		return Metadata{}, err
	}
	m.TokenEndpointAuthMethod = "none"
	m.GrantTypes = []string{"authorization_code", "refresh_token"}
	m.ResponseTypes = []string{"code"}
	m.ClientName = clean(m.ClientName, 100)
	if m.ClientURI != "" && !isHTTPS(m.ClientURI) {
		m.ClientURI = ""
	}
	return m, nil
}

// checkGrants refuses any grant or response type but the code flow's.
func checkGrants(m Metadata) error {
	for _, grant := range m.GrantTypes {
		if grant != "authorization_code" && grant != "refresh_token" {
			return invalid("grant_types may only hold authorization_code and refresh_token")
		}
	}
	for _, kind := range m.ResponseTypes {
		if kind != "code" {
			return invalid("response_types may only hold code")
		}
	}
	return nil
}

// checkRedirects accepts https URLs, http on a loopback address, and, for a
// native client, a private-use scheme (RFC 8252 §7.1). Reverse-domain form
// is what RFC 8252 recommends, but Cursor and VS Code register cursor:// and
// vscode://, so only schemes a browser would execute are refused.
// A fragment is refused everywhere (RFC 6749 §3.1.2).
func checkRedirects(uris []string, applicationType string) error {
	if len(uris) == 0 || len(uris) > 10 {
		return invalidRedirect("redirect_uris must hold between one and ten URIs")
	}
	for _, raw := range uris {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Fragment != "" || parsed.User != nil || !parsed.IsAbs() {
			return invalidRedirect("redirect_uris holds an invalid URI")
		}
		if !redirectAllowed(parsed, applicationType) {
			return invalidRedirect("redirect_uris holds a URI a " + applicationType + " client may not use")
		}
	}
	return nil
}

// redirectAllowed is the per-URI half of checkRedirects.
func redirectAllowed(parsed *url.URL, applicationType string) bool {
	switch parsed.Scheme {
	case "https":
		return parsed.Host != ""
	case "http":
		return IsLoopback(parsed.Hostname())
	}
	return applicationType == "native" && !browserScheme(parsed.Scheme)
}

// browserScheme reports a scheme a browser would run or read rather than
// hand to an app, so a redirect to one is an attack on the browser.
func browserScheme(scheme string) bool {
	switch scheme {
	case "javascript", "data", "file", "vbscript", "blob", "about":
		return true
	}
	return false
}

// isHTTPS reports whether raw is an absolute https URL.
func isHTTPS(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}
