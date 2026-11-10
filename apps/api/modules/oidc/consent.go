package oidc

import (
	"context"
	"errors"
	"log/slog"
	"net/url"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
)

// ConsentPath is the page where a person allows or refuses a third-party
// client. The callback sends the browser there, with the auth request, until
// a live grant covers what the flow asks for.
const ConsentPath = "/connect"

// consentURL is the consent page for one pending flow.
func consentURL(authRequestID string) string {
	return ConsentPath + "?auth_request=" + url.QueryEscape(authRequestID)
}

// consentOwed reports whether a finished flow still needs the person's
// consent: it is a third-party client's, and no live grant for that client
// and resource covers every scope the flow asks for. A suite client never
// needs one, because the suite vouches for it.
func (s *Storage) consentOwed(ctx context.Context, row schemas.AuthRequest) (bool, error) {
	client, err := s.svc.Register.ByClientID(ctx, row.ClientID)
	if err != nil || !client.ThirdParty() {
		return false, err
	}
	if s.svc.Grants == nil || row.UserID == nil {
		return true, nil
	}
	grant, found, err := s.svc.Grants.Live(ctx, *row.UserID, row.ClientID, row.Resource)
	if err != nil {
		return true, err
	}
	return !found || !grant.Covers(row.Scopes), nil
}

// refuseResourceless refuses a third-party flow with no resource. A third
// party exists here to reach one app's MCP server; a sign-in with nothing to
// reach would hand it the person's profile for no purpose.
func refuseResourceless(client schemas.Client) error {
	if client.ThirdParty() {
		return zoidc.ErrInvalidTarget().WithDescription("a resource is required")
	}
	return nil
}

// requireLiveGrant refuses a third-party refresh once its grant is revoked.
// Revoking already ends the refresh tokens; this holds even for one that a
// future code path forgets to end.
func (s *Storage) requireLiveGrant(ctx context.Context, row schemas.RefreshToken) error {
	client, err := s.svc.Register.ByClientID(ctx, row.ClientID)
	if errors.Is(err, clients.ErrUnknownClient) {
		return err
	}
	if err != nil || !client.ThirdParty() {
		return err
	}
	if s.svc.Grants == nil || len(row.Audiences) != 1 {
		return errors.New("oidc: no grant backs this refresh")
	}
	_, found, err := s.svc.Grants.Live(ctx, row.UserID, row.ClientID, row.Audiences[0])
	if err == nil && !found {
		err = errors.New("oidc: the grant was revoked")
	}
	return err
}

// touchGrant stamps last_used_at on the grant behind a resource-bound token,
// which is what the connected apps page shows. A failure is only logged: the
// token is already issued and the stamp is a convenience.
func (s *Storage) touchGrant(ctx context.Context, token schemas.AccessToken) {
	if s.svc.Grants == nil || len(token.Audiences) != 1 || !clients.LooksLikeResource(token.Audiences[0]) {
		return
	}
	if err := s.svc.Grants.Touch(ctx, token.Subject, token.ClientID, token.Audiences[0], token.IssuedAt); err != nil {
		s.logger.Warn("registre: could not stamp a grant", slog.Any("error", err))
	}
}
