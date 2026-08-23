// Package oidc is registre's OpenID Provider: the storage zitadel/oidc issues
// tokens through, and the routes it answers on.
//
// The protocol itself is not implemented here and must not be. PKCE
// verification, authorization code binding, client authentication and token
// issuance are the attack-prone parts, and they live in audited library code
// on purpose — SPEC §3. What this package owns is the register's half: who is
// a client, what they may ask for, which key signs it, and what is recorded.
package oidc

import (
	"crypto/sha256"

	"github.com/go-chi/chi/v5"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// codeKeyContext separates the auth code encryption key from the root key that
// seals the signing keys. One root key, two purposes, two derived keys: a
// compromise of the shorter-lived one must not hand over the other.
const codeKeyContext = "registre/auth-code"

// scopes is what discovery advertises. It is the list phase 0 can honour, not
// the list registre will end up with: offline_access waits for refresh tokens
// in phase 1, and the roles scope waits for the claim behind it in phase 2.
// Advertising a scope with nothing behind it is how porte's own startup guard
// describes the worst failure mode available — every role check denying,
// silently, with no error anywhere.
var scopes = []string{oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail}

// ProviderConfig is what building the provider needs beyond its storage.
type ProviderConfig struct {
	// Issuer is the URL registre answers as. It is static rather than
	// derived from the request Host: a provider that trusts the Host header
	// signs tokens for whatever issuer an attacker asks it to.
	Issuer string

	// RootKey is the process root key. The auth code key is derived from
	// it rather than configured separately, so there is one secret to
	// rotate rather than two to forget.
	RootKey []byte

	// Insecure tolerates a plain-http issuer. Development only; env.Load
	// is where that is decided.
	Insecure bool
}

// NewProvider builds the OpenID Provider over storage.
//
// It takes no logger, and that is not an omission. op.WithLogger is a no-op in
// zitadel/oidc v3 — it accepts a *slog.Logger and discards it — and every line
// the library emits goes through the package-level slog instead. The only way
// to reach those lines is slog.SetDefault, which main does. Passing a logger
// here would read as if it worked, which is how the provider's refusals ended
// up in a different format, at a different level, and out of Journal.
func NewProvider(storage *Storage, cfg ProviderConfig) (*op.Provider, error) {
	options := []op.Option{op.WithCORSOptions(nil)}
	if cfg.Insecure {
		options = append(options, op.WithAllowInsecure())
	}
	return op.NewProvider(&op.Config{
		CryptoKey:       sha256.Sum256(append([]byte(codeKeyContext), cfg.RootKey...)),
		CodeMethodS256:  true,
		AuthMethodPost:  true,
		SupportedScopes: scopes,
		SupportedClaims: op.DefaultSupportedClaims,
	}, storage, op.StaticIssuer(cfg.Issuer), options...)
}

// Mount registers the provider's endpoints on router.
//
// Each path is registered by name rather than mounting the provider at the
// root, because the provider's own router carries a /ready of its own and
// would shadow tronc's — the readiness probe the container's health check
// calls. Discovery is the one path whose location is fixed by the
// specification; the rest are wherever discovery says they are.
func Mount(router chi.Router, provider *op.Provider) {
	for _, path := range Paths(provider) {
		router.Handle(path, provider)
	}
}

// Paths is every path the provider answers on, in the order discovery lists
// them. Exported because the tests assert the mounted set against it rather
// than against a second copy that would drift.
//
// The list includes endpoints phase 0 does not implement, device authorization
// among them. zitadel/oidc writes them into the discovery document whether or
// not their storage exists, so leaving them unmounted turns a clean OIDC
// "unsupported grant" into a 404 from whatever else claims the path — an error
// that says the provider is missing rather than the grant.
func Paths(provider *op.Provider) []string {
	return []string{
		oidc.DiscoveryEndpoint,
		provider.KeysEndpoint().Relative(),
		provider.TokenEndpoint().Relative(),
		provider.IntrospectionEndpoint().Relative(),
		provider.RevocationEndpoint().Relative(),
		provider.UserinfoEndpoint().Relative(),
		provider.AuthorizationEndpoint().Relative(),
		provider.AuthorizationEndpoint().Relative() + "/callback",
		provider.EndSessionEndpoint().Relative(),
		provider.DeviceAuthorizationEndpoint().Relative(),
	}
}
