package oidc

import (
	"context"
	"log/slog"

	jose "github.com/go-jose/go-jose/v4"
	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
)

// The rest of op.Storage. The authorization code flow moved to
// storage_auth.go and refresh tokens to storage_refresh.go; what stays here
// refuses rather than pretends.
// Each is a typed OIDC error rather than a plain one because the library maps
// an untyped error to server_error and answers 500 — "registre broke" and
// "registre does not do that yet" are different facts, and only one of them
// should page anybody.
func unsupported(flow string) *zoidc.Error {
	return zoidc.ErrUnsupportedGrantType().
		WithDescription("%s is not supported yet", flow)
}

// TerminateSession ends every live provider session the user holds and
// announces each of them over the back channel, to exactly the apps those
// sessions had reached.
//
// The coarse sweep is deliberate, matching what porte itself does when it
// receives one of these tokens: per-app precision needs session bookkeeping
// per relying party, and nobody has asked for it yet. An app without a
// registered logout URI hears nothing — that gap is visible in the register,
// where closing it is a field, not a migration.
//
// The back-channel announcements are fired on their own goroutine rather than
// inline: delivery fans out to every app the session reached, each with its own
// timeout, and an end_session request is answered by a browser that should not
// wait N apps × 5s for other people's webhooks. Best-effort is the documented
// contract, so the goroutine carries its own background context and logs
// failures exactly as the synchronous version did.
//
// A request that names nobody — no id_token_hint, no client — ends nothing
// and is refused: ending nothing and redirecting as if it worked would be a
// lie with a 302 on it.
func (s *Storage) TerminateSession(ctx context.Context, userID, _ string) error {
	if userID == "" {
		return zoidc.ErrInvalidRequest().WithDescription("end session names no user")
	}
	if s.svc.Logout == nil {
		if _, err := s.svc.Users.EndSessions(ctx, userID, s.now()); err != nil {
			return zoidc.ErrServerError().WithParent(err)
		}
		return nil
	}
	reached, err := s.svc.Users.EndSessions(ctx, userID, s.now())
	if err != nil {
		return zoidc.ErrServerError().WithParent(err)
	}
	go func() {
		if err := s.svc.Logout.Fire(context.Background(), userID, reached); err != nil {
			s.logger.Warn("registre: back-channel logout incomplete", slog.Any("error", err))
		}
	}()
	return nil
}

// SetUserinfoFromScopes is deprecated in the contract and superseded by the
// roles-aware implementation in userinfo.go. It stays declared there only.

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
