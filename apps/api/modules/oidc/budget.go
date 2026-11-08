package oidc

import (
	"net/http"
	"strings"
)

// DefaultPublicTokenLimit and DefaultPublicTokenBurst bound how fast one
// source address may call the token endpoint without a client secret.
//
// Such a call costs no argon2 verification, only an indexed lookup of a code,
// a refresh token or a device code, so it can afford a far larger budget than
// a confidential client's. It needs one: every claude.ai user's MCP
// connector refreshes from Anthropic's egress range (160.79.104.0/21), every
// ten minutes, under a single CIMD client id, so the address budget of a
// confidential app would throttle all of them together.
const (
	DefaultPublicTokenLimit = 20
	DefaultPublicTokenBurst = 200
)

// carriesSecret reports whether a token request authenticates with a client
// secret, which is the request that costs an argon2 verification.
func carriesSecret(r *http.Request) bool {
	if strings.HasPrefix(r.Header.Get("Authorization"), "Basic ") {
		return true
	}
	if err := r.ParseForm(); err != nil {
		return true
	}
	return r.PostForm.Get("client_secret") != "" || r.PostForm.Get("client_assertion") != ""
}

// splitBudget sends a request that carries a secret through secret and every
// other one through public. A malformed body counts as carrying a secret, so
// it meets the tighter of the two.
func splitBudget(secret, public func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		secretNext, publicNext := secret(next), public(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if carriesSecret(r) {
				secretNext.ServeHTTP(w, r)
				return
			}
			publicNext.ServeHTTP(w, r)
		})
	}
}
