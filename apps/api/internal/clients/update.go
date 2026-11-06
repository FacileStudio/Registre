package clients

import (
	"context"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// UpdateForm writes the columns the dashboard's client form owns and nothing
// else: name, grant types, scopes, audiences, redirect URIs, visible roles,
// the back-channel logout URI and the deletion URI.
//
// It exists beside Upsert because Upsert writes the whole declarative half of
// a row, and the form does not carry the whole half: public,
// access_token_ttl, managed, directory_sync_uri and token_exchange are not on
// it. Saving the form through Upsert once turned the public facile-cli
// client confidential and broke every CLI login. The rule is the register's
// own: write only the columns a declaration owns.
func (s *Service) UpdateForm(ctx context.Context, form schemas.Client) (schemas.Client, error) {
	result := s.db.WithContext(ctx).Model(&schemas.Client{}).
		Where("client_id = ?", form.ClientID).
		Updates(map[string]any{
			"name":                   form.Name,
			"grant_types":            form.GrantTypes,
			"scopes":                 form.Scopes,
			"audiences":              form.Audiences,
			"redirect_uris":          form.RedirectURIs,
			"visible_roles":          form.VisibleRoles,
			"backchannel_logout_uri": form.BackchannelLogoutURI,
			"deletion_uri":           form.DeletionURI,
			"updated_at":             time.Now().UTC(),
		})
	if result.Error != nil {
		return schemas.Client{}, fmt.Errorf("clients: update %s: %w", form.ClientID, result.Error)
	}
	if result.RowsAffected != 1 {
		return schemas.Client{}, ErrUnknownClient
	}
	return s.rawByClientID(ctx, form.ClientID)
}
