package clients

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/internal/metadoc"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// ThirdPartyTokenTTL is the access token lifetime of every third-party
// client. A JWT cannot be taken back, so this is how long a revoked
// connection keeps working at the most: ten minutes, then the refresh is
// refused.
const ThirdPartyTokenTTL = 10 * time.Minute

// ErrFirstParty is a third-party registration that would overwrite a client
// the suite declared.
var ErrFirstParty = errors.New("clients: that client id belongs to a first-party client")

// ThirdPartyRow is the register row for a client registered from metadata: a
// public application with the code and refresh grants, no secret, and the
// third-party token lifetime. Its own scopes are the OIDC ones; the MCP
// resource scopes every client may ask for next to their resource.
func ThirdPartyRow(clientID, registration string, meta metadoc.Metadata, expires *time.Time) schemas.Client {
	name := meta.ClientName
	if name == "" {
		name = clientID
	}
	return schemas.Client{
		ClientID: clientID, Name: name, Kind: schemas.KindApplication, Public: true,
		GrantTypes:     schemas.StringList{"authorization_code", "refresh_token"},
		Scopes:         schemas.StringList{"openid", "profile", "email", "offline_access"},
		RedirectURIs:   schemas.StringList(meta.RedirectURIs),
		AccessTokenTTL: int64(ThirdPartyTokenTTL / time.Second),
		AccessPolicy:   schemas.AccessEveryone,
		Registration:   registration, ClientURI: meta.ClientURI, MetadataExpiresAt: expires,
	}
}

// Stored reads a client by id whatever its state, so a caller can tell a
// disabled third party, which must stay disabled, from one never seen.
func (s *Service) Stored(ctx context.Context, clientID string) (schemas.Client, bool, error) {
	row, err := s.rawByClientID(ctx, clientID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return schemas.Client{}, false, nil
	}
	return row, err == nil, err
}

// SaveThirdParty creates or refreshes a third-party client from its row. It
// writes only what metadata may change, never disabled_at or the access
// policy, so an operator's decision about a client outlives its next fetch.
func (s *Service) SaveThirdParty(ctx context.Context, row schemas.Client) (schemas.Client, error) {
	existing, found, err := s.Stored(ctx, row.ClientID)
	if err != nil {
		return schemas.Client{}, err
	}
	if !found {
		return s.createThirdParty(ctx, row)
	}
	if !existing.ThirdParty() {
		return schemas.Client{}, ErrFirstParty
	}
	err = s.db.WithContext(ctx).Model(&schemas.Client{}).Where("id = ?", existing.ID).Updates(map[string]any{
		"name": row.Name, "redirect_uris": row.RedirectURIs, "client_uri": row.ClientURI,
		"metadata_expires_at": row.MetadataExpiresAt, "updated_at": time.Now().UTC(),
	}).Error
	if err != nil {
		return schemas.Client{}, fmt.Errorf("clients: refresh %s: %w", row.ClientID, err)
	}
	return s.rawByClientID(ctx, row.ClientID)
}

// createThirdParty inserts a new third-party row with a fresh subject.
func (s *Service) createThirdParty(ctx context.Context, row schemas.Client) (schemas.Client, error) {
	subject, err := ids.New()
	if err != nil {
		return schemas.Client{}, fmt.Errorf("clients: mint a subject: %w", err)
	}
	now := time.Now().UTC()
	row.Subject, row.CreatedAt, row.UpdatedAt = subject, now, now
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return schemas.Client{}, fmt.Errorf("clients: register %s: %w", row.ClientID, err)
	}
	return row, nil
}
