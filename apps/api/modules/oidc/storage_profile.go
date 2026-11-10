package oidc

import (
	"context"
	"errors"
	"fmt"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/i18n"
	"gorm.io/gorm"
)

// boundToResource reports whether a token's only audience is a registered MCP
// resource, the one population of access tokens that carries the profile.
func (s *Storage) boundToResource(ctx context.Context, audience []string) (bool, error) {
	if len(audience) != 1 || !clients.LooksLikeResource(audience[0]) {
		return false, nil
	}
	_, err := s.svc.Register.ByResource(ctx, audience[0])
	if errors.Is(err, clients.ErrUnknownResource) {
		return false, nil
	}
	return err == nil, err
}

// provisioningClaims are the claims an app needs to create the person's
// account on a first MCP call exactly as its web login would: what the ID
// token carries under openid email profile, minus everything an account does
// not need. The email and email_verified ride only together and only when
// registre holds the address as verified, because porte provisions from a
// bearer token on a verified address alone. A disabled or unknown subject
// gets nothing, so no app can create an account for one.
//
// They are not gated on the email and profile scopes. The audience is the app
// itself, which receives the same claims at every web login, and the token's
// holder could read them from userinfo with this very token.
func (s *Storage) provisioningClaims(ctx context.Context, subject string) (map[string]any, error) {
	if s.svc.Users == nil {
		return nil, nil
	}
	user, err := s.svc.Users.ByID(ctx, subject)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("oidc: read the user for a resource token: %w", err)
	}
	if !user.Enabled() {
		return nil, nil
	}
	claims := map[string]any{"locale": i18n.Normalize(user.Locale)}
	if user.EmailVerified && user.Email != "" {
		claims["email"] = user.Email
		claims["email_verified"] = true
	}
	if user.Name != "" {
		claims["name"] = user.Name
	}
	if picture := s.pictureFor(user.ID); picture != "" {
		claims["picture"] = picture
	}
	return claims, nil
}

// withProvisioningClaims adds the profile to claims when the token is bound
// to an MCP resource, and leaves every other token as it was.
func (s *Storage) withProvisioningClaims(
	ctx context.Context, claims map[string]any, audience []string, subject string,
) (map[string]any, error) {
	bound, err := s.boundToResource(ctx, audience)
	if err != nil || !bound {
		return claims, err
	}
	profile, err := s.provisioningClaims(ctx, subject)
	if err != nil {
		return nil, err
	}
	if claims == nil {
		claims = map[string]any{}
	}
	for name, value := range profile {
		claims[name] = value
	}
	return claims, nil
}
