package admin

import (
	"net/url"

	"github.com/FacileStudio/Registre/apps/api/schemas"
	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
)

type newClient struct {
	ClientID             string   `json:"client_id"`
	Name                 string   `json:"name"`
	Kind                 string   `json:"kind"`
	GrantTypes           []string `json:"grant_types"`
	Scopes               []string `json:"scopes"`
	Audiences            []string `json:"audiences"`
	RedirectURIs         []string `json:"redirect_uris"`
	VisibleRoles         []string `json:"visible_roles"`
	BackchannelLogoutURI string   `json:"backchannel_logout_uri"`
}

func (c newClient) invalid() (string, bool) {
	if c.ClientID == "" || c.Name == "" {
		return "client_id and name are required", false
	}
	switch c.Kind {
	case schemas.KindApplication:
		if len(c.RedirectURIs) == 0 {
			return "an application needs at least one redirect_uri, or it can never finish a login", false
		}
	case schemas.KindServiceAccount:
		if len(c.RedirectURIs) > 0 {
			return "a service account has no browser flow, so it takes no redirect_uri", false
		}
	default:
		return "kind must be service_account or application", false
	}
	if bad, ok := firstBadURI(append(c.RedirectURIs, c.BackchannelLogoutURI)); !ok {
		return "not an absolute http or https URL: " + bad, false
	}
	return c.grantsFit()
}

func (c newClient) grantsFit() (string, bool) {
	allowed := map[string]bool{}
	for _, grant := range defaultGrants(c.Kind) {
		allowed[grant] = true
	}
	for _, grant := range c.GrantTypes {
		if !allowed[grant] {
			return "a " + c.Kind + " cannot use the " + grant + " grant", false
		}
	}
	return "", true
}

func firstBadURI(candidates []string) (string, bool) {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Host == "" ||
			(parsed.Scheme != "https" && parsed.Scheme != "http") {
			return candidate, false
		}
	}
	return "", true
}

func (c newClient) declaration() schemas.Client {
	grants, scopes := c.GrantTypes, c.Scopes
	if len(grants) == 0 {
		grants = defaultGrants(c.Kind)
	}
	if len(scopes) == 0 && c.Kind == schemas.KindApplication {
		scopes = []string{"openid", "profile", "email"}
	}
	return schemas.Client{
		ClientID: c.ClientID, Name: c.Name, Kind: c.Kind,
		GrantTypes:   schemas.StringList(grants),
		Scopes:       schemas.StringList(scopes),
		Audiences:    schemas.StringList(c.Audiences),
		RedirectURIs: schemas.StringList(c.RedirectURIs),
		VisibleRoles: schemas.StringList(c.VisibleRoles),

		BackchannelLogoutURI: c.BackchannelLogoutURI,
	}
}

func defaultGrants(kind string) []string {
	if kind == schemas.KindApplication {
		return []string{string(zoidc.GrantTypeCode), string(zoidc.GrantTypeRefreshToken)}
	}
	return []string{string(zoidc.GrantTypeClientCredentials)}
}
