package oidc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	jose "github.com/go-jose/go-jose/v4"
	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"gorm.io/gorm"
)

// Config is what the storage needs beyond its stores.
type Config struct {
	// Issuer is the URL registre answers as, stamped into every token.
	Issuer string

	// AccessTokenTTL is the lifetime a client that sets none of its own
	// gets. Short by default: a machine token is renewed by a program, so
	// the cost of a short life is nothing and the benefit is the window a
	// leaked one is useful for.
	AccessTokenTTL time.Duration

	// Now is the clock the storage reads. Nil means time.Now in UTC.
	Now func() time.Time
}

// Storage is registre's implementation of the contract zitadel/oidc issues
// tokens through. It is the load-bearing file of the whole repository: every
// decision about who gets a token, for how long, and naming whom, is made here
// and nowhere else.
// Services bundles what the storage reads besides its database, so the
// constructor stays under filet's parameter ceiling and callers can pass nil
// for a machine-only test. It is a bag of dependencies, not a decision: none
// of the three changes what a token means.
type Services struct {
	Keys     *keys.Service
	Register *clients.Service
	Users    *identity.Service
}

// Storage is registre's implementation of the contract zitadel/oidc issues
// tokens through. It is the load-bearing file of the whole repository: every
// decision about who gets a token, for how long, and naming whom, is made here
// and nowhere else.
type Storage struct {
	db     *gorm.DB
	svc    Services
	cfg    Config
	logger *slog.Logger
	now    func() time.Time
}

// NewStorage builds the storage over the register, the key service and the
// identity service.
func NewStorage(db *gorm.DB, svc Services, cfg Config, logger *slog.Logger) *Storage {
	clock := cfg.Now
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return &Storage{db: db, svc: svc, cfg: cfg, logger: logger, now: clock}
}

// Health reports whether the register is reachable.
func (s *Storage) Health(ctx context.Context) error {
	handle, err := s.db.WithContext(ctx).DB()
	if err != nil {
		return err
	}
	return handle.PingContext(ctx)
}

// SigningKey returns the key tokens are signed with right now.
func (s *Storage) SigningKey(ctx context.Context) (op.SigningKey, error) {
	return s.svc.Keys.Signing(ctx)
}

// SignatureAlgorithms reports what discovery advertises signing under.
func (s *Storage) SignatureAlgorithms(context.Context) ([]jose.SignatureAlgorithm, error) {
	return s.svc.Keys.Algorithms(), nil
}

// KeySet returns every key a relying party should accept: the active one,
// anything published ahead of it, and anything retired but still in retention.
func (s *Storage) KeySet(ctx context.Context) ([]op.Key, error) {
	return s.svc.Keys.Set(ctx)
}

// GetClientByClientID returns a client for the flow machinery to validate
// against. It never returns a disabled row, so disabling a client in the
// register takes effect on the next request rather than the next restart.
func (s *Storage) GetClientByClientID(ctx context.Context, clientID string) (op.Client, error) {
	row, err := s.svc.Register.ByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return client{row: row, ttl: row.TokenLifetime(s.cfg.AccessTokenTTL)}, nil
}

// AuthorizeClientIDSecret verifies a client secret at the token, introspection
// and revocation endpoints.
func (s *Storage) AuthorizeClientIDSecret(ctx context.Context, clientID, secret string) error {
	_, err := s.svc.Register.Authenticate(ctx, clientID, secret)
	return err
}

// ClientCredentials authenticates a machine principal at the token endpoint.
func (s *Storage) ClientCredentials(ctx context.Context, clientID, secret string) (op.Client, error) {
	row, err := s.svc.Register.Authenticate(ctx, clientID, secret)
	if err != nil {
		return nil, err
	}
	return client{row: row, ttl: row.TokenLifetime(s.cfg.AccessTokenTTL)}, nil
}

// ClientCredentialsTokenRequest decides what an authenticated machine gets:
// which scopes, naming which apps as the audience, and for how long.
func (s *Storage) ClientCredentialsTokenRequest(
	ctx context.Context, clientID string, scopes []string,
) (op.TokenRequest, error) {
	row, err := s.svc.Register.ByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	granted, err := grant(row, scopes)
	if err != nil {
		return nil, err
	}
	return tokenRequest{
		clientID: row.ClientID,
		subject:  row.Subject,
		audience: audience(row),
		scopes:   granted,
		lifetime: row.TokenLifetime(s.cfg.AccessTokenTTL),
	}, nil
}

// grant narrows a request to what the register allows, refusing rather than
// silently narrowing. A caller that believes it holds a scope it does not is
// how an agent discovers its permissions at the worst possible moment.
func grant(row schemas.Client, requested []string) ([]string, error) {
	if len(requested) == 0 {
		return row.Scopes, nil
	}
	for _, scope := range requested {
		if !row.Scopes.Contains(scope) {
			return nil, zoidc.ErrInvalidScope().
				WithDescription("client %s is not granted the scope %q", row.ClientID, scope)
		}
	}
	return requested, nil
}

// audience is what lands in the token's aud claim: the apps this client may
// call, or the client itself when the register names none.
func audience(row schemas.Client) []string {
	if len(row.Audiences) == 0 {
		return []string{row.ClientID}
	}
	return row.Audiences
}

// CreateAccessToken records an issued token and returns its jti and expiry.
//
// The token itself is a JWT the apps verify offline against the JWKS, so this
// row is not the credential. It is what makes revocation possible at all: a
// self-contained JWT cannot be taken back, and an agent's credential that
// cannot be taken back is one that has to be waited out.
func (s *Storage) CreateAccessToken(ctx context.Context, request op.TokenRequest) (string, time.Time, error) {
	var clientID, subject string
	var scopes, auds []string
	var lifetime time.Duration
	switch granted := request.(type) {
	case tokenRequest:
		clientID, subject, scopes, auds = granted.clientID, granted.subject, granted.scopes, granted.audience
		lifetime = granted.lifetime
	case op.AuthRequest:
		// A browser flow names the human as the subject and the app
		// that sent them here as the audience.
		clientID, subject = granted.GetClientID(), granted.GetSubject()
		scopes, auds = granted.GetScopes(), granted.GetAudience()
		row, err := s.svc.Register.ByClientID(ctx, clientID)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("oidc: read the client: %w", err)
		}
		lifetime = row.TokenLifetime(s.cfg.AccessTokenTTL)
	default:
		return "", time.Time{}, fmt.Errorf("oidc: %T is not a grant registre issues yet", request)
	}
	id, err := ids.New()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("oidc: mint a token id: %w", err)
	}
	issued := s.now()
	expires := issued.Add(lifetime)
	row := schemas.AccessToken{
		ID: id, ClientID: clientID, Subject: subject,
		Scopes: scopes, Audiences: auds,
		IssuedAt: issued, ExpiresAt: expires,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return "", time.Time{}, fmt.Errorf("oidc: record the token: %w", err)
	}
	return id, expires, nil
}

// RevokeToken takes a token back before it expires.
//
// The client id is part of the match on purpose: RFC 7009 §2.1 requires the
// server to check that the token was issued to the caller, and without it any
// client holding valid credentials could revoke any other client's token.
func (s *Storage) RevokeToken(ctx context.Context, tokenOrTokenID, _, clientID string) *zoidc.Error {
	revoked := s.now()
	err := s.db.WithContext(ctx).Model(&schemas.AccessToken{}).
		Where("id = ? AND client_id = ? AND revoked_at IS NULL", tokenOrTokenID, clientID).
		Update("revoked_at", revoked).Error
	if err != nil {
		return zoidc.ErrServerError().WithParent(err)
	}
	return nil
}

// GetPrivateClaimsFromScopes returns the claims a scope adds to a token.
// There are none yet: the roles claim is SPEC §4's phase 2, and emitting an
// empty one early would make every role check in every app deny silently.
func (s *Storage) GetPrivateClaimsFromScopes(context.Context, string, string, []string) (map[string]any, error) {
	return nil, nil
}

// liveToken reads an issued token that has been neither revoked nor expired.
func (s *Storage) liveToken(ctx context.Context, tokenID string) (schemas.AccessToken, error) {
	var row schemas.AccessToken
	err := s.db.WithContext(ctx).Where("id = ?", tokenID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return schemas.AccessToken{}, errors.New("oidc: no such token")
	}
	if err != nil {
		return schemas.AccessToken{}, fmt.Errorf("oidc: read the token: %w", err)
	}
	if !row.Live(s.now()) {
		return schemas.AccessToken{}, errors.New("oidc: the token is revoked or expired")
	}
	return row, nil
}

// PurgeTokens drops token records that expired more than retention ago. The
// row outlives the token deliberately: revoking something already expired is a
// no-op, but an audit question asked the next morning is not.
func (s *Storage) PurgeTokens(ctx context.Context, retention time.Duration) error {
	cutoff := s.now().Add(-retention)
	err := s.db.WithContext(ctx).Where("expires_at < ?", cutoff).Delete(&schemas.AccessToken{}).Error
	if err != nil {
		return fmt.Errorf("oidc: purge tokens: %w", err)
	}
	return nil
}
