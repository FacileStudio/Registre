package clients

import (
	"context"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// FormColumns are the only columns the dashboard's client form may write.
func FormColumns() []string {
	return []string{
		"name", "grant_types", "scopes", "audiences", "redirect_uris",
		"visible_roles", "backchannel_logout_uri", "deletion_uri",
	}
}

// UpdateForm writes changes, keyed by column, to the client's row, refusing
// any column outside FormColumns. The caller passes only the fields the
// request actually carried, so a field the form omitted keeps its stored
// value.
//
// It exists beside Upsert because Upsert writes the whole declarative half of
// a row and the form does not carry the whole half. Saving the form through
// Upsert once turned the public facile-cli client confidential, and filling
// the omitted grant types with defaults dropped its device grant: both broke
// every CLI login. The rule is the register's own: write only the columns a
// declaration owns, and only the ones it actually states.
func (s *Service) UpdateForm(ctx context.Context, clientID string, changes map[string]any) (schemas.Client, error) {
	allowed := map[string]bool{}
	for _, column := range FormColumns() {
		allowed[column] = true
	}
	updates := map[string]any{"updated_at": time.Now().UTC()}
	for column, value := range changes {
		if !allowed[column] {
			return schemas.Client{}, fmt.Errorf("clients: the form does not own %s", column)
		}
		updates[column] = value
	}
	result := s.db.WithContext(ctx).Model(&schemas.Client{}).Where("client_id = ?", clientID).Updates(updates)
	if result.Error != nil {
		return schemas.Client{}, fmt.Errorf("clients: update %s: %w", clientID, result.Error)
	}
	if result.RowsAffected != 1 {
		return schemas.Client{}, ErrUnknownClient
	}
	return s.rawByClientID(ctx, clientID)
}
