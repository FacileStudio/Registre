// Package clients is the register itself: who may ask registre for a token,
// what they authenticate with, and the ceiling on what they may ask for.
//
// In phase 0 every row is a service account — a machine principal with a
// subject of its own, which is the point of the exercise. An agent on a
// human's credential *is* that human in every audit trail, and revoking one
// revokes both.
package clients

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// ErrUnknownClient is returned for a client that does not exist, is disabled,
// or presented the wrong secret. It is deliberately one error for all three:
// saying which of the three it was is an enumeration oracle.
var ErrUnknownClient = errors.New("clients: unknown client or bad secret")

// Service reads and writes the register.
type Service struct {
	db          *gorm.DB
	credentials *credentials.Verifier
}

// New builds the service over db, hashing and verifying through verifier.
func New(db *gorm.DB, verifier *credentials.Verifier) *Service {
	return &Service{db: db, credentials: verifier}
}

// HashSecret seals a client secret for storage. The register owns client
// credentials, so it owns the one path that turns a plaintext into a hash.
func (s *Service) HashSecret(ctx context.Context, secret string) (string, error) {
	return s.credentials.Hash(ctx, secret)
}

// ByClientID returns an enabled client by the id it authenticates under.
func (s *Service) ByClientID(ctx context.Context, clientID string) (schemas.Client, error) {
	var row schemas.Client
	err := s.db.WithContext(ctx).Where("client_id = ?", clientID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return schemas.Client{}, ErrUnknownClient
	}
	if err != nil {
		return schemas.Client{}, fmt.Errorf("clients: read %s: %w", clientID, err)
	}
	if !row.Enabled() {
		return schemas.Client{}, ErrUnknownClient
	}
	return row, nil
}

// Authenticate verifies a client secret and returns the client behind it.
//
// Every outcome runs the same argon2 verification. A missing client, a
// disabled one and a wrong secret all arrive here holding an empty hash, and
// an empty hash costs what a real one costs — the verifier guarantees that, so
// this function has no early return to forget.
func (s *Service) Authenticate(ctx context.Context, clientID, secret string) (schemas.Client, error) {
	row, err := s.ByClientID(ctx, clientID)
	if err != nil && !errors.Is(err, ErrUnknownClient) {
		return schemas.Client{}, err
	}
	matched, verifyErr := s.credentials.Matches(ctx, secret, row.SecretHash)
	if verifyErr != nil {
		return schemas.Client{}, verifyErr
	}
	if !matched {
		return schemas.Client{}, ErrUnknownClient
	}
	return row, nil
}

// List returns every client, oldest first.
func (s *Service) List(ctx context.Context) ([]schemas.Client, error) {
	var rows []schemas.Client
	if err := s.db.WithContext(ctx).Order("created_at").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("clients: list: %w", err)
	}
	return rows, nil
}

// declared names the columns a declaration owns, and is the whole of what an
// update writes.
//
// The list is explicit and the default is to leave a column alone, because the
// opposite default already produced a bug: writing the whole row cleared
// disabled_at, so an account disabled by hand came back on the next boot with
// its old secret. A revocation that undoes itself is worse than no revocation,
// having been reported as done. Adding a column to the model must not silently
// add it to what a re-apply overwrites.
//
// The secret is the one column with three cases rather than two. A declaration
// that carries one writes it; one that carries none leaves the stored hash
// alone, so a re-apply cannot lock a client out; and a declaration that says
// the client is public erases it, because a public client that kept a usable
// secret would be one every honest reading of the register says has none.
func declared(row schemas.Client) map[string]any {
	fields := map[string]any{
		"name":                   row.Name,
		"kind":                   row.Kind,
		"grant_types":            row.GrantTypes,
		"scopes":                 row.Scopes,
		"audiences":              row.Audiences,
		"access_token_ttl":       row.AccessTokenTTL,
		"managed":                row.Managed,
		"public":                 row.Public,
		"redirect_uris":          row.RedirectURIs,
		"backchannel_logout_uri": row.BackchannelLogoutURI,
		"directory_sync_uri":     row.DirectorySyncURI,
		"visible_roles":          row.VisibleRoles,
		"token_exchange":         row.TokenExchange,
		"updated_at":             time.Now().UTC(),
	}
	switch {
	case row.Public:
		fields["secret_hash"] = ""
	case row.SecretHash != "":
		fields["secret_hash"] = row.SecretHash
	}
	return fields
}

// Upsert replaces the whole declarative half of the row keyed by client id:
// name, kind, grant_types, scopes, audiences, access_token_ttl, managed,
// redirect_uris, backchannel_logout_uri, visible_roles and token_exchange are each written
// exactly as given, including as empty.
//
// Every caller must therefore read-modify-write first — fetch the existing row
// through ByClientID or rawByClientID, change only the fields it owns, and
// pass the complete result back here. An upsert built from a partially-filled
// struct silently blanks whatever was left zero, and the JSONB columns
// (redirect_uris, backchannel_logout_uri, visible_roles) are the ones most
// painful to lose: an application loses every allowed redirect URI and stops
// logging anyone in; an app loses its logout endpoint and stops being told
// about logouts; a role whitelist widening or vanishing changes what tokens
// may claim without any error anywhere.
//
// A row that already exists keeps its subject, because that is what every app
// matched the account on and re-keying it orphans the client everywhere it has
// ever authenticated. It keeps its disabled_at so a revocation cannot be
// undone by a re-apply, and its secret when the caller supplies none and does
// not declare it public.
func (s *Service) Upsert(ctx context.Context, row schemas.Client) (schemas.Client, error) {
	existing, err := s.rawByClientID(ctx, row.ClientID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := time.Now().UTC()
		row.CreatedAt, row.UpdatedAt = now, now
		if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
			return schemas.Client{}, fmt.Errorf("clients: create %s: %w", row.ClientID, err)
		}
		return row, nil
	}
	if err != nil {
		return schemas.Client{}, err
	}

	err = s.db.WithContext(ctx).Model(&schemas.Client{}).
		Where("id = ?", existing.ID).Updates(declared(row)).Error
	if err != nil {
		return schemas.Client{}, fmt.Errorf("clients: update %s: %w", row.ClientID, err)
	}
	return s.rawByClientID(ctx, row.ClientID)
}

func (s *Service) rawByClientID(ctx context.Context, clientID string) (schemas.Client, error) {
	var row schemas.Client
	err := s.db.WithContext(ctx).Where("client_id = ?", clientID).Take(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return schemas.Client{}, fmt.Errorf("clients: read %s: %w", clientID, err)
	}
	return row, err
}

func (s *Service) Disable(ctx context.Context, clientID string, at time.Time) error {
	result := s.db.WithContext(ctx).Model(&schemas.Client{}).
		Where("client_id = ?", clientID).
		Updates(map[string]any{"disabled_at": at, "updated_at": at})
	if result.Error != nil {
		return fmt.Errorf("clients: disable %s: %w", clientID, result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrUnknownClient
	}
	return nil
}

func (s *Service) Enable(ctx context.Context, clientID string) error {
	result := s.db.WithContext(ctx).Model(&schemas.Client{}).
		Where("client_id = ?", clientID).
		Updates(map[string]any{"disabled_at": nil, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return fmt.Errorf("clients: enable %s: %w", clientID, result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrUnknownClient
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, clientID string) error {
	existing, err := s.rawByClientID(ctx, clientID)
	if err != nil {
		return err
	}
	if existing.Managed {
		return fmt.Errorf("clients: cannot delete seed-managed client %s", clientID)
	}
	result := s.db.WithContext(ctx).Where("client_id = ?", clientID).Delete(&schemas.Client{})
	if result.Error != nil {
		return fmt.Errorf("clients: delete %s: %w", clientID, result.Error)
	}
	return nil
}
