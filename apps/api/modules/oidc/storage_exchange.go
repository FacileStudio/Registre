package oidc

import (
	"context"
	"errors"
	"github.com/FacileStudio/Registre/apps/api/internal/access"
	"log/slog"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"gorm.io/gorm"
)

// ExchangeLifetime is the longest a delegated token lives. Five minutes is the
// whole revocation story for a token an app verifies offline: nothing can take
// a JWT back, so the window it is useful for is the window it was issued for.
const ExchangeLifetime = 5 * time.Minute

var _ op.TokenExchangeStorage = (*Storage)(nil)

// ValidateTokenExchangeRequest decides whether a client may trade a human's
// token for a delegated one, and narrows what it asks for to what it is allowed.
//
// The caller has already authenticated by the time this runs, through the same
// argon2 verification as every other grant, so a refusal here costs what an
// acceptance costs. The grant itself is checked here too: the library's token
// handler dispatches on grant_type without asking the client whether it holds
// it, so a client row that never declared the exchange would otherwise reach
// the subject checks. The library has also verified the
// subject token's signature, issuer and expiry. What is left is the register's
// half, in the order the contract states it: who is asking, what they hold,
// where they want to go and what they want there.
func (s *Storage) ValidateTokenExchangeRequest(ctx context.Context, request op.TokenExchangeRequest) error {
	caller, err := s.svc.Register.ByClientID(ctx, request.GetClientID())
	if err != nil {
		return zoidc.ErrInvalidClient().WithParent(err)
	}
	if caller.Public || !caller.GrantTypes.Contains(string(zoidc.GrantTypeTokenExchange)) {
		return zoidc.ErrUnauthorizedClient().WithDescription("the client may not use this grant")
	}
	if err := exchangeShape(request); err != nil {
		return err
	}
	if err := s.checkSubjectToken(ctx, request, caller); err != nil {
		return err
	}
	grant, err := s.exchangeGrant(ctx, request, caller)
	if err != nil {
		return err
	}
	scopes, err := narrowExchangeScopes(grant, request.GetScopes())
	if err != nil {
		return err
	}
	request.SetCurrentScopes(scopes)
	request.SetRequestedTokenType(zoidc.AccessTokenType)
	return nil
}

// checkSubjectToken refuses a subject token that is not a live, human, first
// hand token issued to this caller.
//
// The row is read rather than trusted from the JWT: the signature proves
// registre minted it, and only the row says it has not been revoked since, who
// it was issued to and whether it is itself a delegation. A subject that is not
// a user row, or a disabled one, is refused, which is also what keeps a service
// account's token from being traded: its subject is a client, not a person.
func (s *Storage) checkSubjectToken(ctx context.Context, request op.TokenExchangeRequest, caller schemas.Client) error {
	token, err := s.liveToken(ctx, request.GetExchangeSubjectTokenIDOrToken())
	if err != nil {
		return refusedSubject()
	}
	if token.ClientID != caller.ClientID || !token.Audiences.Contains(caller.ClientID) || token.Actor != "" {
		return refusedSubject()
	}
	user, err := s.svc.Users.ByID(ctx, token.Subject)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return refusedSubject()
	}
	if err != nil {
		return zoidc.ErrServerError().WithParent(err)
	}
	if !user.Enabled() {
		return refusedSubject()
	}
	return nil
}

// checkAudienceAccess refuses minting a delegated token for an app the
// subject may not use: an exchange must never reach an app the human could not
// sign in to themselves. app is the client id exchangeApp resolved, so an MCP
// resource URI is checked against the app that runs it.
func (s *Storage) checkAudienceAccess(ctx context.Context, request op.TokenExchangeRequest, app string) error {
	token, err := s.liveToken(ctx, request.GetExchangeSubjectTokenIDOrToken())
	if err != nil {
		return refusedSubject()
	}
	if err := access.New(s.db, "").Require(ctx, app, token.Subject); err != nil {
		return zoidc.ErrAccessDenied().WithDescription("the user may not use %s", app).WithParent(err)
	}
	return nil
}

// CreateTokenExchangeRequest is called once a request has validated. Nothing is
// stored here: the audit line needs the token id, which does not exist yet, so
// it is written where the token is recorded.
func (s *Storage) CreateTokenExchangeRequest(context.Context, op.TokenExchangeRequest) error {
	return nil
}

// GetPrivateClaimsFromTokenExchangeRequest returns the claims that make a token
// a delegation: the scopes it carries, who is acting, and the subject's email.
//
// scope is a space-separated string, the shape RFC 9068 gives it and the one
// porte reads. act names the calling client as its only member (RFC 8693 §4.1);
// the human stays in sub. email is the subject's address, present only when
// registre holds it as verified: the audience maps a delegated caller to its
// own account by address, and an unverified one must never be the key that
// does it. Only exchanged tokens carry it; a plain access token and an ID token
// are unchanged. roles are left out on purpose: a delegated token carries the
// least that works, and an app that wants a role asks for a scope. An exchange
// for an MCP resource also carries the provisioning claims, so Bulle's first
// call to an app's MCP server can create the account there.
func (s *Storage) GetPrivateClaimsFromTokenExchangeRequest(
	ctx context.Context, request op.TokenExchangeRequest,
) (map[string]any, error) {
	user, err := s.svc.Users.ByID(ctx, request.GetSubject())
	if err != nil {
		return nil, zoidc.ErrServerError().WithParent(err)
	}
	claims := map[string]any{
		"scope": strings.Join(request.GetScopes(), " "),
		"act":   map[string]any{"sub": request.GetClientID()},
	}
	if user.EmailVerified && user.Email != "" {
		claims["email"] = user.Email
	}
	return s.withProvisioningClaims(ctx, claims, request.GetAudience(), request.GetSubject())
}

// SetUserinfoFromTokenExchangeRequest is never reached, because an ID token is
// never issued by exchange. Failing is the safe answer if that ever changes
// without this being revisited.
func (s *Storage) SetUserinfoFromTokenExchangeRequest(
	context.Context, *zoidc.UserInfo, op.TokenExchangeRequest,
) error {
	return errors.New("oidc: token exchange does not issue ID tokens")
}

// mintExchanged records a delegated token. Its life is the shorter of
// ExchangeLifetime and what is left of the subject token, so a delegation can
// never outlive the login it came from; the subject is read again here rather
// than trusted from validation, which ran a moment ago.
func (s *Storage) mintExchanged(ctx context.Context, request op.TokenExchangeRequest) (string, time.Time, error) {
	subject, err := s.liveToken(ctx, request.GetExchangeSubjectTokenIDOrToken())
	if err != nil {
		return "", time.Time{}, refusedSubject()
	}
	lifetime := ExchangeLifetime
	if left := subject.ExpiresAt.Sub(s.now()); left < lifetime {
		lifetime = left
	}
	id, expires, err := s.mintAccessToken(ctx, lifetime, request, request.GetClientID(), request.GetSubject())
	if err != nil {
		return "", time.Time{}, err
	}
	s.logger.Info("registre: token exchanged",
		slog.String("actor", request.GetClientID()),
		slog.String("subject", request.GetSubject()),
		slog.Any("audience", request.GetAudience()),
		slog.Any("scopes", request.GetScopes()),
		slog.String("jti", id),
		slog.String("subject_jti", subject.ID))
	return id, expires, nil
}

// actorOf names the client acting on the subject's behalf when a token is
// issued by exchange, and nobody otherwise.
func actorOf(request op.TokenRequest) string {
	if exchanged, ok := request.(op.TokenExchangeRequest); ok {
		return exchanged.GetClientID()
	}
	return ""
}
