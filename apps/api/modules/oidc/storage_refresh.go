package oidc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/zitadel/oidc/v3/pkg/op"
	"gorm.io/gorm"
)

// Refresh tokens, the second slice of phase 1. A grant lives exactly as long
// as its session would have, rotates on every redemption so a replayed token
// kills the copy the thief holds, and is stored only as a digest because the
// value crosses redirects and access logs like any other bearer credential.

// refreshTokenHash is SHA-256, hex encoded — the same treatment an
// authorization code gets in storage_auth.go, for the same reason.
func refreshTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateAccessAndRefreshTokens issues the access token and, alongside it, a
// renewable grant bound to the same human, app and authentication event.
//
// Before anything is issued the subject is resolved against the register of
// humans: a disabled one gets nothing, whatever their grants say. This is
// defense in depth — EndSessions already revokes a disabled user's grants —
// because a future call site that mints without ending sessions would
// otherwise resurrect exactly the bug this file exists not to have. A subject
// with no user row is a machine principal and passes untouched.
//
// Rotation is unconditional: every redemption ends the presented grant and
// mints a fresh one. That is what makes a stolen refresh token detectable
// rather than silently usable forever — the next legitimate redemption finds
// its grant already revoked.
func (s *Storage) CreateAccessAndRefreshTokens(
	ctx context.Context, request op.TokenRequest, currentRefreshToken string,
) (string, string, time.Time, error) {
	principal := grantPrincipal(request)
	clientID, subject := principal.clientID, principal.subject
	if s.svc.Users != nil {
		user, err := s.svc.Users.ByID(ctx, subject)
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
		case err != nil:
			return "", "", time.Time{}, fmt.Errorf("oidc: read the grant's owner: %w", err)
		case !user.Enabled():
			return "", "", time.Time{}, errors.New("oidc: the account is disabled")
		}
	}

	row, err := s.svc.Register.ByClientID(ctx, clientID)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("oidc: read the client: %w", err)
	}
	accessID, expires, err := s.mintAccessToken(ctx, row.TokenLifetime(s.cfg.AccessTokenTTL), request, clientID, subject)
	if err != nil {
		return "", "", time.Time{}, err
	}
	value, err := s.rotateRefreshToken(ctx, request, currentRefreshToken, clientID, subject)
	if err != nil {
		return "", "", time.Time{}, err
	}
	return accessID, value, expires, nil
}

// principal is the half of op.TokenRequest that names an app and the human or
// machine it speaks for. The interface exists because three unrelated concrete
// types satisfy it — the browser flow's request, a redeemed refresh grant and
// a device flow's state — and matching on the behaviour rather than on the
// list means the next grant registre gains does not need three switches
// amended in three files, one of which will be missed.
type principal interface {
	GetClientID() string
	GetSubject() string
}

// attested is the other half: when the subject authenticated and how. The
// same three types carry it, and a grant that cannot say loses nothing but
// the defaults below.
type attested interface {
	GetAuthTime() time.Time
	GetAMR() []string
}

// grantPrincipal is what every grant registre issues names: the app and the
// human or machine it speaks for. op.TokenRequest carries the subject,
// audience and scopes; the client id rides only on the concrete types.
func grantPrincipal(request op.TokenRequest) struct{ clientID, subject string } {
	named, ok := request.(principal)
	if !ok {
		return struct{ clientID, subject string }{}
	}
	return struct{ clientID, subject string }{named.GetClientID(), named.GetSubject()}
}

// rotateRefreshToken retires the presented grant and records its successor.
// A first issuance passes an empty current token; a redemption passes the one
// being spent, which must be live and belong to exactly this client and user.
func (s *Storage) rotateRefreshToken(
	ctx context.Context, request op.TokenRequest, current, clientID, subject string,
) (string, error) {
	now := s.now()
	if current != "" {
		digest := refreshTokenHash(current)
		result := s.db.WithContext(ctx).Model(&schemas.RefreshToken{}).
			Where("id = ? AND client_id = ? AND user_id = ? AND revoked_at IS NULL", digest, clientID, subject).
			Update("revoked_at", now)
		if result.Error != nil {
			return "", fmt.Errorf("oidc: spend the refresh token: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return "", errors.New("oidc: the refresh token is unknown, spent or expired")
		}
	}
	principal := grantPrincipal(request)
	authTime, amr := time.Now().UTC(), []string{"pwd"}
	if proven, ok := request.(attested); ok {
		authTime = proven.GetAuthTime()
		amr = proven.GetAMR()
	}
	value, err := ids.Secret()
	if err != nil {
		return "", fmt.Errorf("oidc: mint a refresh token: %w", err)
	}
	row := schemas.RefreshToken{
		ID:        refreshTokenHash(value),
		UserID:    principal.subject,
		ClientID:  principal.clientID,
		Scopes:    schemas.StringList(request.GetScopes()),
		Audiences: schemas.StringList(request.GetAudience()),
		AMR:       schemas.StringList(amr),
		AuthTime:  authTime,
		CreatedAt: now,
		ExpiresAt: now.Add(s.refreshTTL()),
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return "", fmt.Errorf("oidc: record the refresh token: %w", err)
	}
	return value, nil
}

// refreshTTL is how long a refresh grant may be redeemed. Zero config falls
// back to thirty days, which is also what env.Load validates against.
func (s *Storage) refreshTTL() time.Duration {
	if s.cfg.RefreshTokenTTL <= 0 {
		return 30 * 24 * time.Hour
	}
	return s.cfg.RefreshTokenTTL
}

// TokenRequestByRefreshToken resolves a presented grant back into the flow it
// was minted from. The digest is the primary key, so this is a point lookup;
// liveness is checked here rather than by the caller because "expired" and
// "unknown" must be indistinguishable to whoever is guessing.
func (s *Storage) TokenRequestByRefreshToken(ctx context.Context, refreshToken string) (op.RefreshTokenRequest, error) {
	var row schemas.RefreshToken
	err := s.db.WithContext(ctx).
		Where("id = ?", refreshTokenHash(refreshToken)).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("oidc: no such refresh token")
	}
	if err != nil {
		return nil, fmt.Errorf("oidc: read the refresh token: %w", err)
	}
	if !row.Live(s.now()) {
		return nil, errors.New("oidc: the refresh token is revoked or expired")
	}
	return &refreshRequest{row: row}, nil
}

// GetRefreshTokenInfo answers whether a token string is one of ours. It is
// how revocation tells a refresh grant from an access token, and answering
// op.ErrInvalidRefreshToken — not a lookup miss dressed up differently — is
// the contract that lets the caller try the other interpretation.
func (s *Storage) GetRefreshTokenInfo(ctx context.Context, _, token string) (string, string, error) {
	var row schemas.RefreshToken
	err := s.db.WithContext(ctx).
		Where("id = ?", refreshTokenHash(token)).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", "", op.ErrInvalidRefreshToken
	}
	if err != nil {
		return "", "", fmt.Errorf("oidc: read the refresh token: %w", err)
	}
	if !row.Live(s.now()) {
		return "", "", op.ErrInvalidRefreshToken
	}
	return row.UserID, row.ID, nil
}

// refreshRequest adapts the stored grant to the contract the refresh grant
// drives. SetCurrentScopes narrows later redemptions: the library applies it
// to the copy handed out here, and the narrowed list is what the next
// CreateAccessAndRefreshTokens stores.
type refreshRequest struct {
	row schemas.RefreshToken
}

func (r refreshRequest) GetAMR() []string                  { return r.row.AMR }
func (r refreshRequest) GetAudience() []string             { return r.row.Audiences }
func (r refreshRequest) GetAuthTime() time.Time            { return r.row.AuthTime }
func (r refreshRequest) GetClientID() string               { return r.row.ClientID }
func (r refreshRequest) GetScopes() []string               { return r.row.Scopes }
func (r refreshRequest) GetSubject() string                { return r.row.UserID }
func (r *refreshRequest) SetCurrentScopes(scopes []string) { r.row.Scopes = scopes }
