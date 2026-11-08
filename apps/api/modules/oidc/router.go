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
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/devices"
	"github.com/FacileStudio/Registre/apps/api/internal/limits"
	"github.com/go-chi/chi/v5"
	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// codeKeyContext separates the auth code encryption key from the root key that
// seals the signing keys. One root key, two purposes, two derived keys: a
// compromise of the shorter-lived one must not hand over the other.
const codeKeyContext = "registre/auth-code"

// scopes is what discovery advertises. It is the list phase 1 and 2 can
// honour: offline_access since refresh tokens shipped, roles since the claim
// behind it exists and is filtered per client. Advertising a scope with
// nothing behind it is how porte's own startup guard describes the worst
// failure mode available — every role check denying, silently, with no error
// anywhere.
var scopes = []string{
	zoidc.ScopeOpenID, zoidc.ScopeProfile, zoidc.ScopeEmail,
	zoidc.ScopeOfflineAccess, scopeRoles, scopeConnections,
}

// scopeRoles is the scope that asks for the roles claim in a token.
const scopeRoles = "roles"

// scopeConnections is the scope that asks for the connections claim: the
// subjects of the external accounts, such as Discord, the person has linked.
const scopeConnections = "connections"

// DefaultDeviceStartLimit and DefaultDeviceStartBurst bound how fast one
// source address may open device flows.
//
// The endpoint is unauthenticated by construction — a CLI has nothing to
// authenticate with yet — and every call writes a row holding a low-entropy
// user code. Flooding it is therefore a way to raise the collision odds the
// user code's entropy budget assumes, which is a subtler attack than filling
// a table. Starting a login is a once-a-day act for a human, so the budget is
// generous by any honest measure and still five orders of magnitude below
// what would matter.
const (
	DefaultDeviceStartLimit = 5
	DefaultDeviceStartBurst = 10
)

// DefaultTokenLimit and DefaultTokenBurst bound how fast one source address
// may call the token endpoint. Every call there is client-authenticated, and a
// confidential client's secret is an argon2 verification of its own, so an
// unauthenticated flood of bad secrets costs the server about 145 ms of CPU
// each. A real app calls it once per sign-in and once per refresh.
const (
	DefaultTokenLimit = 1
	DefaultTokenBurst = 10
)

// deviceAuthorization is the flow's shape as the library mints it: how long a
// flow lives, how often the CLI is told to poll, where the human goes, and
// what a user code looks like.
//
// The user code's alphabet, length and entropy are internal/devices'
// decision and are documented there. They are read from that package rather
// than from zitadel's own equivalent constants so the explanation and the
// codes actually minted cannot drift apart.
func deviceAuthorization() op.DeviceAuthorizationConfig {
	return op.DeviceAuthorizationConfig{
		Lifetime:     devices.Lifetime,
		PollInterval: devices.PollInterval,
		UserFormPath: devices.VerificationPath,
		UserCode: op.UserCodeConfig{
			CharSet:      devices.Alphabet,
			CharAmount:   devices.CodeLength,
			DashInterval: devices.GroupSize,
		},
	}
}

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
		CryptoKey:             sha256.Sum256(append([]byte(codeKeyContext), cfg.RootKey...)),
		CodeMethodS256:        true,
		AuthMethodPost:        true,
		GrantTypeRefreshToken: true,
		SupportedScopes:       scopes,
		SupportedClaims:       append(op.DefaultSupportedClaims, scopeRoles, scopeConnections),
		DeviceAuthorization:   deviceAuthorization(),
	}, storage, op.StaticIssuer(cfg.Issuer), options...)
}

// Mount registers the provider's endpoints on router.
//
// Each path is registered by name rather than mounting the provider at the
// root, because the provider's own router carries a /ready of its own and
// would shadow tronc's — the readiness probe the container's health check
// calls. Discovery is the one path whose location is fixed by the
// specification; the rest are wherever discovery says they are.
//
// Device authorization is the one endpoint that gets a per-address budget.
// Every other path here either authenticates its caller before it does any
// work or reads something public; that one is unauthenticated and writes a row
// carrying a low-entropy user code, so the rate at which it may be called is
// part of RFC 8628 §5.2's arithmetic rather than a nicety.
//
// /authorize and its callback go through binding, which ties every browser
// flow to the browser that started it and the session that finished it. See
// Binding for why the library's own callback cannot be mounted bare.
func Mount(router chi.Router, provider *op.Provider, binding Binding) {
	MountWithTokenLimit(router, provider, binding, DefaultTokenLimit, DefaultTokenBurst)
}

// MountWithTokenLimit is Mount with an explicit per-address budget on the token
// endpoint for requests that carry a client secret. The token endpoint also
// answers 503 with Retry-After when every argon2 slot stayed busy, instead of
// the library's invalid_client, because telling an app its secret is wrong
// when the server is merely busy is a lie that ends in a rotated secret.
func MountWithTokenLimit(router chi.Router, provider *op.Provider, binding Binding, tokenLimit float64, tokenBurst int) {
	MountWithLimits(router, provider, binding, Limits{
		Token: tokenLimit, TokenBurst: tokenBurst,
		PublicToken: DefaultPublicTokenLimit, PublicTokenBurst: DefaultPublicTokenBurst,
	})
}

// Limits are the per-address budgets of the token endpoint: one for requests
// that carry a client secret, which cost an argon2 verification, and one for
// public clients, which do not.
type Limits struct {
	Token            float64
	TokenBurst       int
	PublicToken      float64
	PublicTokenBurst int
}

// MountWithLimits is Mount with explicit token endpoint budgets. It also
// mounts RFC 8414 metadata next to discovery, the RFC 8707 resource gate on
// the token endpoint, and RFC 9207 iss on every authorization response.
func MountWithLimits(router chi.Router, provider *op.Provider, binding Binding, limits Limits) {
	routes := endpointHandlers(provider, binding, limits)
	for _, path := range Paths(provider) {
		handler, ok := routes[path]
		if !ok {
			handler = provider
		}
		router.Handle(path, handler)
	}
	router.Get(MetadataPath, authorizationServerMetadata(routes[zoidc.DiscoveryEndpoint]).ServeHTTP)
}

// endpointHandlers are the paths that are not the bare provider.
func endpointHandlers(provider *op.Provider, binding Binding, budgets Limits) map[string]http.Handler {
	tokenBudget := splitBudget(limits.PerAddress(budgets.Token, budgets.TokenBurst),
		limits.PerAddress(budgets.PublicToken, budgets.PublicTokenBurst))
	authorize := provider.AuthorizationEndpoint().Relative()
	return map[string]http.Handler{
		zoidc.DiscoveryEndpoint: filteredDiscovery(provider),
		provider.DeviceAuthorizationEndpoint().Relative(): limits.PerAddress(
			DefaultDeviceStartLimit, DefaultDeviceStartBurst)(provider),
		provider.TokenEndpoint().Relative(): tokenBudget(busyAs503(binding.Storage.targets(provider))),
		authorize:                           withIssuer(provider.IssuerFromRequest, binding.authorize(provider)),
		authorize + "/callback": withIssuer(provider.IssuerFromRequest,
			binding.callback(provider, provider)),
	}
}

// PublicPaths are the endpoints a browser-based OAuth client calls across
// origins, which CORS opens to any origin without credentials.
func PublicPaths(provider *op.Provider) []string {
	return []string{
		zoidc.DiscoveryEndpoint,
		MetadataPath,
		provider.KeysEndpoint().Relative(),
		provider.TokenEndpoint().Relative(),
		provider.RevocationEndpoint().Relative(),
	}
}

// Paths is every path the provider answers on, in the order discovery lists
// them. Exported because the tests assert the mounted set against it rather
// than against a second copy that would drift.
//
// The list once included endpoints registre did not implement, and that is
// still why it is exhaustive: zitadel/oidc writes them into the discovery
// document whether or not their storage exists, so leaving one unmounted turns
// a clean OIDC "unsupported grant" into a 404 from whatever else claims the
// path — an error that says the provider is missing rather than the grant.
func Paths(provider *op.Provider) []string {
	return []string{
		zoidc.DiscoveryEndpoint,
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
