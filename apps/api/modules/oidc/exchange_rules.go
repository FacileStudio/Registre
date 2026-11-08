package oidc

import (
	"github.com/FacileStudio/Registre/apps/api/schemas"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// The refusals are fixed values, one per class, so that nothing in a response
// says which of several checks failed. The caller holds the token it sent and
// learns nothing from the answer it could not learn by trying.
func refusedSubject() *zoidc.Error {
	return zoidc.ErrInvalidGrant().WithDescription("the subject token cannot be exchanged")
}

func refusedTarget() *zoidc.Error {
	return zoidc.ErrInvalidTarget().WithDescription("the audience cannot be requested")
}

func refusedScope() *zoidc.Error {
	return zoidc.ErrInvalidScope().WithDescription("the scope cannot be requested for this audience")
}

func refusedShape(why string) *zoidc.Error {
	return zoidc.ErrInvalidRequest().WithDescription("%s", why)
}

// exchangeShape refuses the parts of RFC 8693 registre does not offer: any
// subject but an access token, an actor token, a resource indicator and any
// issued type but an access token.
func exchangeShape(request op.TokenExchangeRequest) error {
	if request.GetExchangeSubjectTokenType() != zoidc.AccessTokenType {
		return refusedSubject()
	}
	if request.GetExchangeActor() != "" || request.GetExchangeActorTokenIDOrToken() != "" {
		return refusedShape("an actor token is not accepted")
	}
	if len(request.GetResourses()) > 0 {
		return refusedTarget()
	}
	if issued := request.GetRequestedTokenType(); issued != "" && issued != zoidc.AccessTokenType {
		return refusedShape("only an access token can be issued")
	}
	return nil
}

// exchangeAllowance finds the grant that names app, the one app the exchange
// is for once exchangeApp has resolved it.
func exchangeAllowance(caller schemas.Client, app string) (schemas.ExchangeGrant, error) {
	if app == caller.ClientID {
		return schemas.ExchangeGrant{}, refusedTarget()
	}
	grant, ok := caller.TokenExchange.For(app)
	if !ok {
		return schemas.ExchangeGrant{}, refusedTarget()
	}
	return grant, nil
}

// narrowExchangeScopes refuses rather than narrows, for the reason grant does:
// a caller that believes it holds a scope it does not is how an agent finds out
// at the worst moment. Asking for nothing asks for the whole allowance.
func narrowExchangeScopes(grant schemas.ExchangeGrant, requested []string) ([]string, error) {
	if len(requested) == 0 {
		return grant.Scopes, nil
	}
	for _, scope := range requested {
		if !contains(grant.Scopes, scope) {
			return nil, refusedScope()
		}
	}
	return requested, nil
}
