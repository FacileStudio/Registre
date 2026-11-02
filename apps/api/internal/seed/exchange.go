package seed

import (
	"fmt"
	"strings"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
)

// exchangeGrantType is the grant a client holds once it declares any
// token_exchange entry. It is derived rather than listed in grant_types: an
// application that names audiences to exchange for has said it wants the grant,
// and a second place to say so is a place for the two to disagree.
const exchangeGrantType = string(zoidc.GrantTypeTokenExchange)

// ExchangeGrant is one audience an application may trade a human's token for,
// and the scopes it may ask for there.
//
// The audience is a client_id declared in this same file, and it is written
// into the issued token's aud claim byte for byte. Registre resolves no names:
// an app verifying the token compares aud against its own OIDC_MACHINE_AUDIENCE
// with equality, so the two strings must be the same string. The suite
// standard is the app's client_id.
type ExchangeGrant struct {
	Audience string   `yaml:"audience"`
	Scopes   []string `yaml:"scopes"`
}

// exchangeScopeActions are the verbs a delegated scope may end in. write exists
// so the vocabulary is fixed before anything grants it.
var exchangeScopeActions = []string{"read", "write"}

// validateExchange rejects a token_exchange block that could not be applied,
// across every application at once because an audience is only meaningful
// against the other clients in the file.
func (f File) validateExchange() error {
	names := map[string]string{}
	for _, application := range f.Applications {
		names[application.ClientID] = application.Name
	}
	for index, application := range f.Applications {
		if len(application.TokenExchange) == 0 {
			continue
		}
		where := fmt.Sprintf("applications[%d] (%s)", index, application.ClientID)
		if err := application.validateExchange(where, names); err != nil {
			return err
		}
	}
	return nil
}

// validateExchange rejects one application's token_exchange block.
func (a Application) validateExchange(where string, names map[string]string) error {
	if a.Public {
		return fmt.Errorf("%s: a public client cannot exchange tokens — "+
			"the exchange authenticates the caller, and a public client has nothing to authenticate with", where)
	}
	seen := map[string]bool{}
	for index, grant := range a.TokenExchange {
		entry := fmt.Sprintf("%s: token_exchange[%d]", where, index)
		if err := a.validateGrant(entry, grant, names, seen); err != nil {
			return err
		}
		seen[grant.Audience] = true
	}
	return nil
}

// validateGrant rejects one audience entry.
func (a Application) validateGrant(entry string, grant ExchangeGrant, names map[string]string, seen map[string]bool) error {
	name, declared := names[grant.Audience]
	switch {
	case grant.Audience == "":
		return fmt.Errorf("%s: audience is required", entry)
	case grant.Audience == a.ClientID:
		return fmt.Errorf("%s: an application cannot exchange for itself", entry)
	case !declared:
		return fmt.Errorf("%s: audience %q is not an application declared in this file", entry, grant.Audience)
	case seen[grant.Audience]:
		return fmt.Errorf("%s: audience %q is declared twice", entry, grant.Audience)
	case len(grant.Scopes) == 0:
		return fmt.Errorf("%s: at least one scope is required — an empty allowance would make every exchange refuse", entry)
	}
	prefix := exchangePrefix(name)
	if prefix == "" {
		return fmt.Errorf("%s: audience %q has no name to derive a scope prefix from", entry, grant.Audience)
	}
	for _, scope := range grant.Scopes {
		if !exchangeScopeOK(prefix, scope) {
			return fmt.Errorf("%s: scope %q must be %s.read or %s.write", entry, scope, prefix, prefix)
		}
	}
	return nil
}

// exchangePrefix is the namespace of the scopes delegated for an app: its name,
// lower case. The apps compare the same string, set as OIDC_SCOPE_PREFIX.
func exchangePrefix(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// exchangeScopeOK reports whether scope is prefix.read or prefix.write.
func exchangeScopeOK(prefix, scope string) bool {
	for _, action := range exchangeScopeActions {
		if scope == prefix+"."+action {
			return true
		}
	}
	return false
}

// exchangeGrants converts the declaration into the stored form.
func (a Application) exchangeGrants() schemas.ExchangeGrants {
	if len(a.TokenExchange) == 0 {
		return nil
	}
	grants := make(schemas.ExchangeGrants, 0, len(a.TokenExchange))
	for _, grant := range a.TokenExchange {
		grants = append(grants, schemas.ExchangeGrant{Audience: grant.Audience, Scopes: grant.Scopes})
	}
	return grants
}

// withExchangeGrant adds the exchange grant to a resolved list when the
// application declares any token_exchange entry.
func (a Application) withExchangeGrant(resolved []string) []string {
	if len(a.TokenExchange) > 0 && !contains(resolved, exchangeGrantType) {
		return append(resolved, exchangeGrantType)
	}
	return resolved
}
