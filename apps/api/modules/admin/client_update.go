package admin

import (
	"encoding/json"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// formEdit is one dashboard edit: the stored client with every field the
// request carried laid over it, and the columns those fields write.
type formEdit struct {
	merged  newClient
	changes map[string]any
}

// readEdit overlays the fields raw carries onto existing. A key the request
// omits keeps the stored value: the drawer never sends grant types, and a
// default there dropped facile-cli's device grant.
func readEdit(existing schemas.Client, raw []byte) (formEdit, error) {
	var present map[string]json.RawMessage
	if err := json.Unmarshal(raw, &present); err != nil {
		return formEdit{}, err
	}
	merged := newClient{
		ClientID: existing.ClientID, Name: existing.Name, Kind: existing.Kind,
		GrantTypes: existing.GrantTypes, Scopes: existing.Scopes, Audiences: existing.Audiences,
		RedirectURIs: existing.RedirectURIs, VisibleRoles: existing.VisibleRoles,
		BackchannelLogoutURI: existing.BackchannelLogoutURI, DeletionURI: existing.DeletionURI,
	}
	targets := map[string]any{
		"name": &merged.Name, "grant_types": &merged.GrantTypes, "scopes": &merged.Scopes,
		"audiences": &merged.Audiences, "redirect_uris": &merged.RedirectURIs,
		"visible_roles": &merged.VisibleRoles, "backchannel_logout_uri": &merged.BackchannelLogoutURI,
		"deletion_uri": &merged.DeletionURI,
	}
	edit := formEdit{changes: map[string]any{}}
	for _, column := range clients.FormColumns() {
		value, ok := present[column]
		if !ok || string(value) == "null" {
			continue
		}
		if err := json.Unmarshal(value, targets[column]); err != nil {
			return formEdit{}, err
		}
		edit.changes[column] = nil
	}
	edit.merged = merged
	for column := range edit.changes {
		edit.changes[column] = merged.column(column)
	}
	return edit, nil
}

// column is the stored form of one field of c.
func (c newClient) column(name string) any {
	switch name {
	case "name":
		return c.Name
	case "grant_types":
		return schemas.StringList(c.GrantTypes)
	case "scopes":
		return schemas.StringList(c.Scopes)
	case "audiences":
		return schemas.StringList(c.Audiences)
	case "redirect_uris":
		return schemas.StringList(c.RedirectURIs)
	case "visible_roles":
		return schemas.StringList(c.VisibleRoles)
	case "backchannel_logout_uri":
		return c.BackchannelLogoutURI
	default:
		return c.DeletionURI
	}
}

// maxClientBody caps a dashboard client edit.
const maxClientBody = 64 << 10

// valid checks the merged client as a whole, and the grant types only when
// the edit states them: a stored grant such as facile-cli's device grant was
// declared by the seed and is not the form's to judge.
func (e formEdit) valid(existing schemas.Client) (string, bool) {
	if e.merged.Name == "" {
		return "name is required", false
	}
	checked := e.merged
	if _, stated := e.changes["grant_types"]; !stated {
		checked.GrantTypes = nil
	} else if len(e.merged.GrantTypes) == 0 {
		return "grant_types cannot be empty: a client with no grant can never get a token", false
	}
	if reason, ok := e.scopesUsable(); !ok {
		return reason, false
	}
	if reason, ok := checked.invalid(); !ok {
		return reason, false
	}
	if e.merged.DeletionURI != "" && e.merged.DeletionURI == existing.DirectorySyncURI {
		return "deletion_uri must not be the directory sync URI: a sync handler accepts any POST and erases nothing", false
	}
	return "", true
}

// scopesUsable refuses an application whose stated scopes are empty or lack
// openid: no sign-in could ever ask for an ID token. The drawer always sends
// scopes, so an emptied field is the easy way to break a client.
func (e formEdit) scopesUsable() (string, bool) {
	if _, stated := e.changes["scopes"]; !stated || e.merged.Kind != schemas.KindApplication {
		return "", true
	}
	for _, scope := range e.merged.Scopes {
		if scope == "openid" {
			return "", true
		}
	}
	return "an application's scopes must include openid", false
}
