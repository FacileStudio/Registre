package oidc

import (
	"errors"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// targets applies RFC 8707 at the token endpoint, before zitadel sees the
// request, because zitadel reads resource only on token exchange and ignores
// it everywhere else.
//
// On a code or refresh grant the parameter may only repeat the resource the
// grant was issued for: it narrows nothing, and asking for another one is
// invalid_target rather than a token for a server nobody consented to. A
// grant whose code or refresh token is unknown passes through, so the library
// answers it with its own invalid_grant.
//
// On client_credentials and the device grant the parameter is ignored, as it
// always was: the register decides a machine's audience, never the request.
//
// On token exchange the resource is rewritten into the audience the library
// mints for, so the issued token's aud is the resource URI. An audience sent
// alongside must name the same app, by client id or by the URI itself.
func (s *Storage) targets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || len(r.Form[resourceParam]) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		if err := s.checkTarget(r); err != nil {
			op.RequestError(w, r, err, s.logger)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// checkTarget dispatches on the grant the request is for.
func (s *Storage) checkTarget(r *http.Request) error {
	asked := r.Form[resourceParam]
	if len(asked) != 1 {
		return refusedResource()
	}
	switch zoidc.GrantType(r.Form.Get("grant_type")) {
	case zoidc.GrantTypeCode:
		return s.checkCodeTarget(r, asked[0])
	case zoidc.GrantTypeRefreshToken:
		return s.checkRefreshTarget(r, asked[0])
	case zoidc.GrantTypeTokenExchange:
		return s.rewriteExchangeTarget(r, asked[0])
	}
	return nil
}

// checkCodeTarget refuses a resource that is not the one the code's flow
// asked for.
func (s *Storage) checkCodeTarget(r *http.Request, resource string) error {
	digest, _ := codeHash(r.Form.Get("code"))
	var row schemas.AuthRequest
	err := s.db.WithContext(r.Context()).Select("resource").Where("code = ?", digest).Take(&row).Error
	if err != nil {
		return nil
	}
	if row.Resource != resource {
		return refusedResource()
	}
	return nil
}

// checkRefreshTarget refuses a resource that is not the one the refresh
// grant was minted for.
func (s *Storage) checkRefreshTarget(r *http.Request, resource string) error {
	var row schemas.RefreshToken
	err := s.db.WithContext(r.Context()).Select("audiences").
		Where("id = ?", refreshTokenHash(r.Form.Get("refresh_token"))).Take(&row).Error
	if err != nil {
		return nil
	}
	if len(row.Audiences) != 1 || row.Audiences[0] != resource {
		return refusedResource()
	}
	return nil
}

// rewriteExchangeTarget turns an exchange's resource into its audience.
func (s *Storage) rewriteExchangeTarget(r *http.Request, resource string) error {
	owner, err := s.svc.Register.ByResource(r.Context(), resource)
	if errors.Is(err, clients.ErrUnknownResource) {
		return refusedResource()
	}
	if err != nil {
		return zoidc.ErrServerError().WithParent(err)
	}
	for _, audience := range r.Form["audience"] {
		if audience != owner.ClientID && audience != resource {
			return refusedResource()
		}
	}
	for _, form := range []map[string][]string{r.Form, r.PostForm} {
		if form == nil {
			continue
		}
		form["audience"] = []string{resource}
		delete(form, resourceParam)
	}
	return nil
}
