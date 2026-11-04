package directory

import (
	"context"
	"net/http"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/gdpr"
)

// DeletionLister answers the accounts erased since a moment, for one app.
type DeletionLister func(ctx context.Context, clientID string, since time.Time) ([]gdpr.Deleted, error)

// WithDeletions turns on GET /api/directory/deletions. Without it the route
// answers 404.
func (s *Service) WithDeletions(list DeletionLister) *Service {
	s.deletionsFor = list
	return s
}

// deletions answers the calling app's erased accounts since ?since= (RFC
// 3339, default: the whole retention window). An app with no sync URI polls
// here; one with a sync URI may poll here to catch up after downtime.
func (s *Service) deletions(w http.ResponseWriter, r *http.Request) {
	if s.deletionsFor == nil {
		encode(w, map[string]string{"error": "not found"}, http.StatusNotFound)
		return
	}
	app, ok := s.app(w, r)
	if !ok {
		return
	}
	since := time.Now().UTC().Add(-gdpr.NoticeRetention)
	if raw := r.URL.Query().Get("since"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			encode(w, map[string]string{"error": "since must be RFC 3339"}, http.StatusBadRequest)
			return
		}
		since = parsed
	}
	out, err := s.deletionsFor(r.Context(), app.ClientID, since)
	if err != nil {
		s.fail(w, err)
		return
	}
	encode(w, map[string]any{"deleted": out}, http.StatusOK)
}
