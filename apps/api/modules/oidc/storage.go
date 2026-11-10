package oidc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/avatars"
	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/devices"
	"github.com/FacileStudio/Registre/apps/api/internal/grants"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"
	"github.com/FacileStudio/Registre/apps/api/internal/logout"
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

	// RefreshTokenTTL is how long a refresh grant may be redeemed. Zero
	// means the storage's thirty-day fallback; env.Load refuses values
	// shorter than the access token lifetime, which would be a grant that
	// dies before the thing it renews.
	RefreshTokenTTL time.Duration

	// Now is the clock the storage reads. Nil means time.Now in UTC.
	Now func() time.Time
}

// Services bundles what the storage reads besides its database. It exists so
// the constructor stays under filet's parameter ceiling and callers can pass
// nil for a machine-only test. It is a bag of dependencies, not a decision:
// none of them changes what a token means.
type Services struct {
	Keys     *keys.Service
	Register *clients.Service
	Users    *identity.Service
	Logout   *logout.Service
	Devices  *devices.Service
	Avatars  *avatars.Service

	// Connections reads the linked external accounts the connections claim
	// carries, per client. Nil omits the claim.
	Connections ConnectionReader

	// Documents fetches client ID metadata documents. Nil turns CIMD off.
	Documents DocumentFetcher

	// Grants holds what people allowed third-party clients. Nil means no
	// third-party client can finish a flow, because none can be consented.
	Grants *grants.Service
}

// ConnectionReader answers the provider-to-subject map of one user.
type ConnectionReader interface {
	Connections(ctx context.Context, userID, clientID string) (map[string]string, error)
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

	fetches chan struct{}
}

// NewStorage builds the storage over the register, the key service and the
// identity service.
func NewStorage(db *gorm.DB, svc Services, cfg Config, logger *slog.Logger) *Storage {
	clock := cfg.Now
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return &Storage{
		db: db, svc: svc, cfg: cfg, logger: logger, now: clock,
		fetches: make(chan struct{}, maxConcurrentFetches),
	}
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
	row, err := s.lookupClient(ctx, clientID)
	if err != nil {
		return nil, err
	}
	extra, err := s.resourceScopes(ctx)
	if err != nil {
		return nil, err
	}
	return client{row: row, ttl: row.TokenLifetime(s.cfg.AccessTokenTTL), resourceScopes: extra}, nil
}

// AuthorizeClientIDSecret verifies a client secret at the token, introspection
// and revocation endpoints.
func (s *Storage) AuthorizeClientIDSecret(ctx context.Context, clientID, secret string) error {
	_, err := s.svc.Register.Authenticate(ctx, clientID, secret)
	noteBusy(ctx, err)
	return err
}

// ClientCredentials authenticates a machine principal at the token endpoint.
func (s *Storage) ClientCredentials(ctx context.Context, clientID, secret string) (op.Client, error) {
	row, err := s.svc.Register.Authenticate(ctx, clientID, secret)
	if err != nil {
		noteBusy(ctx, err)
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
// CreateAccessToken issues an access token for whichever grant asked. A
// browser flow names the human as the subject and the app that sent them here
// as the audience; client_credentials names the machine principal.
func (s *Storage) CreateAccessToken(ctx context.Context, request op.TokenRequest) (string, time.Time, error) {
	var clientID, subject string
	var lifetime time.Duration
	switch granted := request.(type) {
	case op.TokenExchangeRequest:
		return s.mintExchanged(ctx, granted)
	case tokenRequest:
		clientID, subject = granted.clientID, granted.subject
		lifetime = granted.lifetime
	case principal:
		clientID, subject = granted.GetClientID(), granted.GetSubject()
		row, err := s.svc.Register.ByClientID(ctx, clientID)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("oidc: read the client: %w", err)
		}
		lifetime = row.TokenLifetime(s.cfg.AccessTokenTTL)
	default:
		return "", time.Time{}, fmt.Errorf("oidc: %T is not a grant registre issues yet", request)
	}
	return s.mintAccessToken(ctx, lifetime, request, clientID, subject)
}

// mintAccessToken records one issued access token for whichever grant asked.
func (s *Storage) mintAccessToken(
	ctx context.Context, lifetime time.Duration, request op.TokenRequest, clientID, subject string,
) (string, time.Time, error) {
	id, err := ids.New()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("oidc: mint a token id: %w", err)
	}
	issued := s.now()
	expires := issued.Add(lifetime)
	row := schemas.AccessToken{
		ID: id, ClientID: clientID, Subject: subject,
		Scopes: request.GetScopes(), Audiences: request.GetAudience(),
		IssuedAt: issued, ExpiresAt: expires, Actor: actorOf(request),
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return "", time.Time{}, fmt.Errorf("oidc: record the token: %w", err)
	}
	s.touchGrant(ctx, row)
	return id, expires, nil
}

// RevokeToken takes a token back before it expires.
//
// The client id is part of the match on purpose: RFC 7009 §2.1 requires the
// server to check that the token was issued to the caller, and without it any
// client holding valid credentials could revoke any other client's token.
//
// A refresh grant revokes with it everything minted from it — RFC 7009's
// SHOULD — because leaving live access tokens behind would make the
// revocation advisory rather than actual.
func (s *Storage) RevokeToken(ctx context.Context, tokenOrTokenID, userID, clientID string) *zoidc.Error {
	revoked := s.now()
	err := s.db.WithContext(ctx).Model(&schemas.AccessToken{}).
		Where("id = ? AND client_id = ? AND revoked_at IS NULL", tokenOrTokenID, clientID).
		Update("revoked_at", revoked).Error
	if err != nil {
		return zoidc.ErrServerError().WithParent(err)
	}
	if userID == "" {
		return nil
	}
	err = s.db.WithContext(ctx).Model(&schemas.RefreshToken{}).
		Where("id = ? AND client_id = ? AND user_id = ? AND revoked_at IS NULL", tokenOrTokenID, clientID, userID).
		Update("revoked_at", revoked).Error
	if err != nil {
		return zoidc.ErrServerError().WithParent(err)
	}
	err = s.db.WithContext(ctx).Model(&schemas.AccessToken{}).
		Where("client_id = ? AND subject = ? AND revoked_at IS NULL", clientID, userID).
		Update("revoked_at", revoked).Error
	if err != nil {
		return zoidc.ErrServerError().WithParent(err)
	}
	return nil
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
