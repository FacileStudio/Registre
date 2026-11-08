package oidc

import (
	"net/http"
	"net/url"
)

// withIssuer adds RFC 9207's iss parameter to every authorization response
// the wrapped handler sends: a code, or an error, redirected to the client.
//
// zitadel builds those redirects itself and has no option for iss, so the
// Location header is rewritten at the moment the status is written, which is
// after the library set it and before anything reaches the wire. A redirect
// that carries neither code nor error is registre sending the browser to its
// own login page, and is left alone.
func withIssuer(issuer func(*http.Request) string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&issuerWriter{ResponseWriter: w, issuer: issuer(r)}, r)
	})
}

type issuerWriter struct {
	http.ResponseWriter
	issuer  string
	written bool
}

// WriteHeader stamps iss onto an authorization response's Location.
func (w *issuerWriter) WriteHeader(code int) {
	if !w.written && code >= 300 && code < 400 {
		w.stamp()
	}
	w.written = true
	w.ResponseWriter.WriteHeader(code)
}

// Write marks the header as sent, as net/http does on a first write.
func (w *issuerWriter) Write(p []byte) (int, error) {
	w.written = true
	return w.ResponseWriter.Write(p)
}

// stamp adds iss to the Location when it is an authorization response.
func (w *issuerWriter) stamp() {
	target, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		return
	}
	query := target.Query()
	if (query.Get("code") == "" && query.Get("error") == "") || query.Has("iss") {
		return
	}
	query.Set("iss", w.issuer)
	target.RawQuery = query.Encode()
	w.Header().Set("Location", target.String())
}
