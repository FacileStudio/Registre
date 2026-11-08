package oidc

import (
	"context"
	"errors"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// exchangeTarget is the app an exchange is for, and the scopes its MCP
// resource declares when the audience is that resource rather than the app.
type exchangeTarget struct {
	app            string
	resourceScopes schemas.StringList
}

// narrow limits a grant to the resource's scopes when the target is an MCP
// resource. The grant names the app, and the resource may declare less than
// the app's whole exchange vocabulary.
func (t exchangeTarget) narrow(grant schemas.ExchangeGrant) schemas.ExchangeGrant {
	if t.resourceScopes == nil {
		return grant
	}
	kept := make([]string, 0, len(grant.Scopes))
	for _, scope := range grant.Scopes {
		if t.resourceScopes.Contains(scope) {
			kept = append(kept, scope)
		}
	}
	return schemas.ExchangeGrant{Audience: grant.Audience, Scopes: kept}
}

// exchangeApp resolves the one requested audience. A client id is the app
// itself. An MCP resource URI, which the token endpoint's resource gate
// writes into the audience, resolves to the app that declares it, so Bulle's
// existing grant for an app also covers that app's MCP server and nothing
// new has to be seeded for it.
func (s *Storage) exchangeApp(ctx context.Context, request op.TokenExchangeRequest) (exchangeTarget, error) {
	audience := request.GetAudience()
	if len(audience) != 1 {
		return exchangeTarget{}, refusedTarget()
	}
	if !clients.LooksLikeResource(audience[0]) {
		return exchangeTarget{app: audience[0]}, nil
	}
	owner, err := s.svc.Register.ByResource(ctx, audience[0])
	if errors.Is(err, clients.ErrUnknownResource) {
		return exchangeTarget{}, refusedTarget()
	}
	if err != nil {
		return exchangeTarget{}, zoidc.ErrServerError().WithParent(err)
	}
	return exchangeTarget{app: owner.ClientID, resourceScopes: owner.MCPScopes}, nil
}

// exchangeGrant resolves the requested audience, finds the caller's grant for
// the app behind it, checks the subject may use that app, and narrows the
// grant to the resource's scopes when the audience is an MCP resource.
func (s *Storage) exchangeGrant(
	ctx context.Context, request op.TokenExchangeRequest, caller schemas.Client,
) (schemas.ExchangeGrant, error) {
	target, err := s.exchangeApp(ctx, request)
	if err != nil {
		return schemas.ExchangeGrant{}, err
	}
	grant, err := exchangeAllowance(caller, target.app)
	if err != nil {
		return schemas.ExchangeGrant{}, err
	}
	if err := s.checkAudienceAccess(ctx, request, target.app); err != nil {
		return schemas.ExchangeGrant{}, err
	}
	return target.narrow(grant), nil
}
