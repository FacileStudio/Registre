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
	"authorization_code":              true,
	"refresh_token":                   true,
	"client_credentials":              true,
	string(zoidc.GrantTypeDeviceCode): true,
}

func filteredDiscovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &capturingWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(recorder, r)
		if recorder.code != http.StatusOK || recorder.ignored {
			_, _ = w.Write(recorder.body)
			return
		}
		doc := map[string]any{}
		if err := json.Unmarshal(recorder.body, &doc); err != nil {
			_, _ = w.Write(recorder.body)
			return
		}
		grants, ok := doc["grant_types_supported"].([]any)
		if !ok {
			_, _ = w.Write(recorder.body)
			return
		}
		filtered := make([]any, 0, len(grants))
		for _, grant := range grants {
			name, _ := grant.(string)
			if servedGrants[name] {
				filtered = append(filtered, grant)
			}
		}
		doc["grant_types_supported"] = filtered
		body, err := json.Marshal(doc)
		if err != nil {
			_, _ = w.Write(recorder.body)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
}

type capturingWriter struct {
	http.ResponseWriter
	code    int
	body    []byte
	ignored bool
}

func (w *capturingWriter) WriteHeader(code int) {
	w.code = code
	if code != http.StatusOK {
		w.ignored = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *capturingWriter) Write(p []byte) (int, error) {
	w.body = append(w.body, p...)
	return len(p), nil
}
