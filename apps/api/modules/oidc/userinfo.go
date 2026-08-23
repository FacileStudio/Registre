package oidc

import (
	"context"
	"errors"
	"fmt"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"gorm.io/gorm"
)

// SetIntrospectionFromToken answers what a token is, for a caller entitled to
// ask. An app is entitled when it is an audience of the token or issued it;
// anyone else gets the same inactive answer as for a token that never existed.
func (s *Storage) SetIntrospectionFromToken(
	ctx context.Context, response *zoidc.IntrospectionResponse, tokenID, _, clientID string,
) error {
	row, err := s.liveToken(ctx, tokenID)
	if err != nil {
		return err
	}
	if row.ClientID != clientID && !row.Audiences.Contains(clientID) {
		return errors.New("oidc: the caller is not an audience of this token")
	}
	response.Subject = row.Subject
	response.ClientID = row.ClientID
	response.Scope = zoidc.SpaceDelimitedArray(row.Scopes)
	response.Audience = zoidc.Audience(row.Audiences)
	response.Expiration = zoidc.FromTime(row.ExpiresAt)
	response.IssuedAt = zoidc.FromTime(row.IssuedAt)
	response.TokenType = zoidc.BearerToken
	response.Issuer = s.cfg.Issuer
	response.JWTID = row.ID
	return nil
}

// SetUserinfoFromToken answers the userinfo endpoint. A machine principal has
// a subject and nothing else — no name, no address, no email — so that is what
// it gets, rather than a fabricated profile. A human's profile claims come
// from their user row; an unknown subject means a machine and answers with the
// subject alone.
func (s *Storage) SetUserinfoFromToken(ctx context.Context, userinfo *zoidc.UserInfo, tokenID, _, _ string) error {
	row, err := s.liveToken(ctx, tokenID)
	if err != nil {
		return err
	}
	userinfo.Subject = row.Subject
	if s.svc.Users == nil {
		return nil
	}
	user, err := s.svc.Users.ByID(ctx, row.Subject)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("oidc: read the user for userinfo: %w", err)
	}
	if !user.Enabled() {
		return errors.New("oidc: the token's owner is disabled")
	}
	userinfo.Email = user.Email
	userinfo.Name = user.Name
	return nil
}
