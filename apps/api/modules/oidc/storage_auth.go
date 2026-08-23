package oidc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"gorm.io/gorm"
)

// authTTL is how long a started browser flow stays valid. It bounds the
// window between "the app sent you to log in" and "you finished", which is
// also the window a phished authorize URL is worth something in.
const authTTL = 10 * time.Minute

// The browser half of op.Storage: phase 1 replaces the refusals in
// storage_pending.go for everything the authorization code flow needs.
// Refresh tokens stay refused — they are the second slice of phase 1.

// CreateAuthRequest records what /authorize was asked and hands back the
// handle the login UI resumes.
//
// zitadel passes the user id from an id_token_hint as the third argument,
// which is not the request id. The id is ours to mint; the hint's subject is
// deliberately ignored, because pre-authenticating a flow on an unverified
// hint would let anyone skip the password by naming themselves in a URL.
func (s *Storage) CreateAuthRequest(ctx context.Context, request *zoidc.AuthRequest, _ string) (op.AuthRequest, error) {
	id, err := ids.New()
	if err != nil {
		return nil, fmt.Errorf("oidc: mint an auth request id: %w", err)
	}
	row := schemas.AuthRequest{
		ID:                  id,
		ClientID:            request.ClientID,
		RedirectURI:         request.RedirectURI,
		Scopes:              schemas.StringList(request.Scopes),
		State:               request.State,
		Nonce:               request.Nonce,
		ResponseType:        string(request.ResponseType),
		CodeChallenge:       request.CodeChallenge,
		CodeChallengeMethod: string(request.CodeChallengeMethod),
		ExpiresAt:           s.now().Add(authTTL),
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, fmt.Errorf("oidc: record the auth request: %w", err)
	}
	return s.wrap(row)
}

// AuthRequestByID reads a pending browser flow by its id.
func (s *Storage) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	var row schemas.AuthRequest
	err := s.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, zoidc.ErrInvalidRequest().WithDescription("no such or expired login request")
	}
	if err != nil {
		return nil, fmt.Errorf("oidc: read the auth request: %w", err)
	}
	return s.wrap(row)
}

// AuthRequestByCode resolves a pending flow from the code an app redeemed.
//
// SaveAuthCode stores the code on the row, so this is a lookup by it. The row
// is deleted here rather than at expiry alone: a spent code that still answers
// AuthRequestByCode is replayable, and RFC 6749 §4.1.2 says a replayed code
// must revoke what it already issued rather than re-issue.
func (s *Storage) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	var row schemas.AuthRequest
	err := s.db.WithContext(ctx).Where("code = ?", code).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, zoidc.ErrInvalidGrant().WithDescription("the authorization code is unknown, spent or expired")
	}
	if err != nil {
		return nil, fmt.Errorf("oidc: resolve the code: %w", err)
	}
	if err := s.db.WithContext(ctx).Delete(&row).Error; err != nil {
		return nil, fmt.Errorf("oidc: burn the code: %w", err)
	}
	if row.ExpiresAt.Before(s.now()) || !row.Done() {
		return nil, zoidc.ErrInvalidGrant().WithDescription("the authorization code is unknown, spent or expired")
	}
	return s.wrap(row)
}

// SaveAuthCode binds an issued code to the pending flow.
func (s *Storage) SaveAuthCode(ctx context.Context, id, code string) error {
	err := s.db.WithContext(ctx).Model(&schemas.AuthRequest{}).
		Where("id = ?", id).
		Update("code", code).Error
	if err != nil {
		return fmt.Errorf("oidc: bind the code: %w", err)
	}
	return nil
}

// DeleteAuthRequest drops a pending browser flow.
func (s *Storage) DeleteAuthRequest(ctx context.Context, id string) error {
	if err := s.db.WithContext(ctx).Delete(&schemas.AuthRequest{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("oidc: drop the auth request: %w", err)
	}
	return nil
}

// wrap turns the row into the contract zitadel/oidc drives the flow through,
// refusing anything expired as if it never existed.
func (s *Storage) wrap(row schemas.AuthRequest) (op.AuthRequest, error) {
	if row.ExpiresAt.Before(s.now()) {
		if err := s.DeleteAuthRequest(context.Background(), row.ID); err != nil {
			s.logger.Warn("registre: failed to drop an expired auth request", slog.Any("error", err))
		}
		return nil, zoidc.ErrInvalidRequest().WithDescription("no such or expired login request")
	}
	return authRequest{row: row}, nil
}

// authRequest adapts the stored row to op.AuthRequest. Every getter reads the
// row; none of them touch the database, so the flow's hot path is memory only.
type authRequest struct {
	row schemas.AuthRequest
}

func (a authRequest) GetID() string          { return a.row.ID }
func (a authRequest) GetClientID() string    { return a.row.ClientID }
func (a authRequest) GetRedirectURI() string { return a.row.RedirectURI }
func (a authRequest) GetScopes() []string    { return a.row.Scopes }
func (a authRequest) GetState() string       { return a.row.State }
func (a authRequest) GetNonce() string       { return a.row.Nonce }
func (a authRequest) GetResponseType() zoidc.ResponseType {
	return zoidc.ResponseType(a.row.ResponseType)
}
func (a authRequest) GetResponseMode() zoidc.ResponseMode { return "" }
func (a authRequest) GetCodeChallenge() *zoidc.CodeChallenge {
	if a.row.CodeChallenge == "" {
		return nil
	}
	method := a.row.CodeChallengeMethod
	if method == "" {
		method = "S256"
	}
	return &zoidc.CodeChallenge{Challenge: a.row.CodeChallenge, Method: zoidc.CodeChallengeMethod(method)}
}

// GetSubject is empty until the human has authenticated; Done gates the flow
// on exactly that.
func (a authRequest) GetSubject() string {
	if a.row.UserID == nil {
		return ""
	}
	return *a.row.UserID
}

func (a authRequest) GetAuthTime() time.Time {
	if a.row.AuthenticatedAt == nil {
		return time.Time{}
	}
	return *a.row.AuthenticatedAt
}

// GetACR and GetAMR describe how strongly someone authenticated. A password
// over TLS is one factor, and saying so plainly is what lets a future WebAuthn
// step upgrade these values without touching any relying party.
func (a authRequest) GetACR() string   { return "" }
func (a authRequest) GetAMR() []string { return []string{"pwd"} }

func (a authRequest) GetAudience() []string { return []string{a.row.ClientID} }

func (a authRequest) Done() bool { return a.row.Done() }
