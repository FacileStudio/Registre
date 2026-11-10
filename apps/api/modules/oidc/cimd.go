package oidc

import (
	"context"
	"log/slog"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/metadoc"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// maxConcurrentFetches caps how many metadata documents registre reads at
// once. /authorize is unauthenticated and a client_id that is a URL makes
// registre fetch it, so without a cap anyone could turn the provider into a
// fetcher of their choosing at the rate of their choosing.
const maxConcurrentFetches = 4

// DocumentFetcher reads client ID metadata documents. *metadoc.Fetcher is the
// production one; nil turns CIMD off.
type DocumentFetcher interface {
	Fetch(ctx context.Context, clientID string) (metadoc.Metadata, time.Time, error)
}

// lookupClient returns the register row behind clientID. A client_id that is
// a metadata document URL is read from the register while its document is
// fresh, and fetched, validated and saved again once it is not. A third party
// an operator disabled stays disabled whatever its document says. When a
// refresh fails, a row already registered keeps working on its last good
// document until the next try, so a client's brief outage does not sign
// everyone out of it.
func (s *Storage) lookupClient(ctx context.Context, clientID string) (schemas.Client, error) {
	if !metadoc.IsDocumentURL(clientID) {
		return s.svc.Register.ByClientID(ctx, clientID)
	}
	row, found, err := s.svc.Register.Stored(ctx, clientID)
	switch {
	case err != nil:
		return schemas.Client{}, err
	case found && !row.Enabled():
		return schemas.Client{}, clients.ErrUnknownClient
	case found && row.MetadataExpiresAt != nil && s.now().Before(*row.MetadataExpiresAt):
		return row, nil
	}
	fresh, err := s.fetchDocument(ctx, clientID)
	if err != nil && found && row.ThirdParty() {
		s.logger.Warn("registre: a client metadata document could not be refreshed, keeping the last one",
			slog.String("client_id", clientID), slog.Any("error", err))
		return row, nil
	}
	return fresh, err
}

// fetchDocument fetches, validates and saves the document at clientID.
func (s *Storage) fetchDocument(ctx context.Context, clientID string) (schemas.Client, error) {
	if s.svc.Documents == nil {
		return schemas.Client{}, clients.ErrUnknownClient
	}
	select {
	case s.fetches <- struct{}{}:
		defer func() { <-s.fetches }()
	case <-ctx.Done():
		return schemas.Client{}, ctx.Err()
	}
	meta, expires, err := s.svc.Documents.Fetch(ctx, clientID)
	if err != nil {
		s.logger.Info("registre: refused a client metadata document",
			slog.String("client_id", clientID), slog.Any("error", err))
		return schemas.Client{}, clients.ErrUnknownClient
	}
	return s.svc.Register.SaveThirdParty(ctx, clients.ThirdPartyRow(clientID, schemas.RegistrationDocument, meta, &expires))
}
