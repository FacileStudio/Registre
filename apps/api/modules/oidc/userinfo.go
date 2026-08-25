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
//
// email_verified is emitted truthfully, which is the capability Authentik
// never had: an operator-minted account is attested by its creator and says
// so, and porte's address fallback finally gets to fire.
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
	userinfo.EmailVerified = zoidc.Bool(user.EmailVerified)
	userinfo.Name = user.Name
	return nil
}

// SetUserinfoFromScopes feeds the ID token's claims.
//
// Profile and email ride in the ID token, reversing the narrower choice this
// file started with. That choice was defensible on its own terms — an ID token
// is logged, cached and forwarded far more often than a userinfo response — but
// it is not compatible with the suite. porte builds its login claims from the
// ID token alone and refuses an empty address, and its one userinfo call sits
// behind RequireAuth where a login cannot reach it. Authentik carried these
// claims in the ID token too, so emitting them is a restoration rather than a
// widening, and the alternative was teaching seven apps to fetch userinfo.
//
// Every claim is scope-gated: an app that asked for neither profile nor email
// still receives neither. roles stays filtered per client by rolesForClient.
//
// A granted roles scope emits the claim even when the list is empty, because
// absent and empty are not the same answer to porte: it reads an absent claim
// as "the provider is missing its scope mapping" and fails the login, while an
// empty list is the honest "this app may see none of your roles". Visibility
// stays opt-in per client either way.
//
// An unknown subject is a machine principal, which has no profile to emit and
// is not an error.
func (s *Storage) SetUserinfoFromScopes(
	ctx context.Context, userinfo *zoidc.UserInfo, userID, clientID string, scopes []string,
) error {
	if s.svc.Users == nil {
		return nil
	}
	wantsProfile := contains(scopes, zoidc.ScopeProfile)
	wantsEmail := contains(scopes, zoidc.ScopeEmail)
	if wantsProfile || wantsEmail {
		user, err := s.svc.Users.ByID(ctx, userID)
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			return nil
		case err != nil:
			return fmt.Errorf("oidc: read the user for the id token: %w", err)
		}
		if wantsEmail {
			userinfo.Email = user.Email
			userinfo.EmailVerified = zoidc.Bool(user.EmailVerified)
		}
		if wantsProfile {
			userinfo.Name = user.Name
		}
	}
	if !contains(scopes, scopeRoles) {
		return nil
	}
	roles, err := s.rolesForClient(ctx, userID, clientID)
	if err != nil {
		return err
	}
	if roles == nil {
		roles = []string{}
	}
	userinfo.AppendClaims(scopeRoles, roles)
	return nil
}
