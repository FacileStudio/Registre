package oidc

import (
	"context"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// The rest of op.Storage: everything the browser flow needs, which phase 0
// deliberately does not have.
//
// They refuse rather than pretend. SPEC §4 orders the phases by attack surface
// and puts client_credentials first precisely because it has no interactive
// flow — no redirect_uri validation, no state, no nonce, no session fixation.
// A half-built authorization endpoint that returned something plausible would
// throw that ordering away, so each of these answers with the grant it is not.

// unsupported names a flow that arrives in a later phase.
//
// It is a typed OIDC error rather than a plain one because the library maps an
// untyped error to server_error and answers 500. "registre broke" and "registre
// does not do that yet" are different facts, and only one of them should page
// anybody.
func unsupported(flow string) *zoidc.Error {
	return zoidc.ErrUnsupportedGrantType().
		WithDescription("%s is not supported yet: registre is at SPEC §4 phase 0, machine tokens only", flow)
}

// CreateAuthRequest starts a browser flow.
func (s *Storage) CreateAuthRequest(context.Context, *zoidc.AuthRequest, string) (op.AuthRequest, error) {
	return nil, unsupported("the authorization code flow")
}

// AuthRequestByID resumes a pending browser flow.
func (s *Storage) AuthRequestByID(context.Context, string) (op.AuthRequest, error) {
	return nil, unsupported("the authorization code flow")
}

// AuthRequestByCode exchanges an authorization code.
func (s *Storage) AuthRequestByCode(context.Context, string) (op.AuthRequest, error) {
	return nil, unsupported("the authorization code flow")
}

// SaveAuthCode binds an authorization code to a pending request.
func (s *Storage) SaveAuthCode(context.Context, string, string) error {
	return unsupported("the authorization code flow")
}

// DeleteAuthRequest drops a pending browser flow.
func (s *Storage) DeleteAuthRequest(context.Context, string) error {
	return unsupported("the authorization code flow")
}

// CreateAccessAndRefreshTokens issues a refresh token alongside an access
// token. Nothing offline_access holds is issued yet.
func (s *Storage) CreateAccessAndRefreshTokens(
	context.Context, op.TokenRequest, string,
) (string, string, time.Time, error) {
	return "", "", time.Time{}, unsupported("refresh tokens")
}

// TokenRequestByRefreshToken redeems a refresh token.
func (s *Storage) TokenRequestByRefreshToken(context.Context, string) (op.RefreshTokenRequest, error) {
	return nil, unsupported("refresh tokens")
}

// GetRefreshTokenInfo reads a refresh token. Answering with
// op.ErrInvalidRefreshToken is the contract's way of saying "that is not one
// of mine", which is exactly true here.
func (s *Storage) GetRefreshTokenInfo(context.Context, string, string) (string, string, error) {
	return "", "", op.ErrInvalidRefreshToken
}

// TerminateSession ends a user's session at the provider. There are no user
// sessions until phase 1, and back-channel logout is phase 2.
func (s *Storage) TerminateSession(context.Context, string, string) error {
	return zoidc.ErrInvalidRequest().
		WithDescription("end session is not supported yet: registre is at SPEC §4 phase 0, machine tokens only")
}

// SetUserinfoFromScopes is deprecated in the contract and superseded by
// SetUserinfoFromToken. It is empty rather than refusing because the flow
// machinery still calls it on paths that work.
func (s *Storage) SetUserinfoFromScopes(context.Context, *zoidc.UserInfo, string, string, []string) error {
	return nil
}

// GetKeyByIDAndClientID returns a client's own public key, for the
// private_key_jwt client authentication method. registre authenticates clients
// by secret; a second mechanism is a decision, not an omission.
func (s *Storage) GetKeyByIDAndClientID(context.Context, string, string) (*jose.JSONWebKey, error) {
	return nil, unsupported("private_key_jwt client authentication")
}

// ValidateJWTProfileScopes validates a JWT bearer assertion. zitadel/oidc
// advertises that grant unconditionally in discovery; refusing here is what
// makes the advertisement untrue in the only direction that is safe.
func (s *Storage) ValidateJWTProfileScopes(context.Context, string, []string) ([]string, error) {
	return nil, unsupported("the JWT bearer grant")
}
