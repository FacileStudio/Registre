package oidc

import (
	"encoding/json"
	"net/http"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
)

// servedGrants is what registre actually implements. zitadel/oidc writes
// implicit and the JWT bearer grant into the discovery document
// unconditionally, and token exchange whenever its flag is on, so advertising
// its document verbatim promises grants that answer only an error. Filtering
// here keeps the document honest without forking the library's handler.
var servedGrants = map[string]bool{
	"authorization_code":                 true,
	"refresh_token":                      true,
	"client_credentials":                 true,
	string(zoidc.GrantTypeTokenExchange): true,
	string(zoidc.GrantTypeDeviceCode):    true,
}

// MetadataPath is where RFC 8414 puts the authorization server metadata of an
// issuer with no path component. MCP clients read it before anything else.
const MetadataPath = "/.well-known/oauth-authorization-server"

// metadataExtras are the members registre adds to both documents: what the
// library cannot know registre does on top of it.
//
// authorization_response_iss_parameter_supported is RFC 9207: every
// authorization response carries iss, which is what lets a client that talks
// to several servers tell a mix-up attack from a real answer. ChatGPT picks
// its redirect URI on this flag.
func metadataExtras() map[string]any {
	return map[string]any{
		"authorization_response_iss_parameter_supported": true,
		"response_types_supported":                       []string{string(zoidc.ResponseTypeCode)},
	}
}

// filteredDiscovery serves the library's discovery document, made honest:
// only the grants registre serves, only the code response type its clients
// may use, and the members in metadataExtras.
func filteredDiscovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &capturingWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(recorder, r)
		if recorder.code != http.StatusOK || recorder.ignored {
			_, _ = w.Write(recorder.body)
			return
		}
		body, err := honestDocument(recorder.body)
		if err != nil {
			_, _ = w.Write(recorder.body)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
}

// authorizationServerMetadata serves RFC 8414 metadata. It is the discovery
// document itself: every member the OIDC document carries is defined for
// RFC 8414 too, and two documents that could drift apart would be two
// answers to "how do I talk to this server".
func authorizationServerMetadata(discovery http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rewritten := r.Clone(r.Context())
		rewritten.URL.Path = zoidc.DiscoveryEndpoint
		rewritten.URL.RawPath = ""
		discovery.ServeHTTP(w, rewritten)
	})
}

// honestDocument applies servedGrants and metadataExtras to a discovery
// document. A document that does not parse is an error, and the caller
// serves the library's bytes unchanged rather than nothing.
func honestDocument(raw []byte) ([]byte, error) {
	doc := map[string]any{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if grants, ok := doc["grant_types_supported"].([]any); ok {
		filtered := make([]any, 0, len(grants))
		for _, grant := range grants {
			if name, _ := grant.(string); servedGrants[name] {
				filtered = append(filtered, grant)
			}
		}
		doc["grant_types_supported"] = filtered
	}
	for name, value := range metadataExtras() {
		doc[name] = value
	}
	return json.Marshal(doc)
}

type capturingWriter struct {
	http.ResponseWriter
	code    int
	body    []byte
	ignored bool
}

// WriteHeader records a status and passes through any that is not 200.
func (w *capturingWriter) WriteHeader(code int) {
	w.code = code
	if code != http.StatusOK {
		w.ignored = true
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write buffers the body so the document can be rewritten before it is sent.
func (w *capturingWriter) Write(p []byte) (int, error) {
	w.body = append(w.body, p...)
	return len(p), nil
}
