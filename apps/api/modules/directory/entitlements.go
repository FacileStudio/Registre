package directory

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/FacileStudio/Registre/apps/api/internal/audit"
	spaces "github.com/FacileStudio/Registre/apps/api/internal/directory"
)

// maxEntitlementsBytes bounds a push. The suite's plans fit in kilobytes.
const maxEntitlementsBytes = 1 << 20

// entitlementsPush is the PUT body. Unknown fields are refused.
type entitlementsPush struct {
	Revision int64                       `json:"revision"`
	Spaces   []spaces.SpaceEntitlements  `json:"spaces"`
	People   []spaces.PersonEntitlements `json:"people"`
}

// WithWriter turns the push on for the one client named writer, which
// is Ardoise's client id, and records every applied push in the audit log.
// An empty writer leaves the route answering 404 to everyone.
func (s *Service) WithWriter(writer string, events *audit.Service) *Service {
	s.writer, s.audit = writer, events
	return s
}

// setEntitlements applies a full entitlements push from the writer.
//
// The order is fixed: disabled answers 404 before any hashing, then the
// caller proves it is an application under the argon2 ceiling (401), then
// that it is the writer (403), and only then is the body read, capped at
// 1 MiB, with unknown fields and malformed values refused (400). A push
// older than the stored revision answers 409 with the current one.
func (s *Service) setEntitlements(w http.ResponseWriter, r *http.Request) {
	if s.writer == "" {
		encode(w, map[string]string{"error": "not found"}, http.StatusNotFound)
		return
	}
	app, ok := s.app(w, r)
	if !ok {
		return
	}
	if app.ClientID != s.writer {
		encode(w, map[string]string{"error": "forbidden"}, http.StatusForbidden)
		return
	}
	var body entitlementsPush
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxEntitlementsBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || decoder.More() {
		encode(w, map[string]string{"error": "malformed entitlements"}, http.StatusBadRequest)
		return
	}
	applied, err := s.spaces.SetEntitlements(r.Context(), body.Revision, body.Spaces, body.People)
	if s.refused(w, err) {
		return
	}
	s.record(r, app.ClientID, body.Revision, applied)
	w.WriteHeader(http.StatusNoContent)
}

// refused answers a failed push and reports whether it did.
func (s *Service) refused(w http.ResponseWriter, err error) bool {
	var stale *spaces.StaleRevisionError
	switch {
	case err == nil:
		return false
	case errors.As(err, &stale):
		encode(w, map[string]any{"error": "stale revision", "current": stale.Current}, http.StatusConflict)
	case errors.Is(err, spaces.ErrInvalidEntitlements):
		encode(w, map[string]string{"error": err.Error()}, http.StatusBadRequest)
	default:
		s.fail(w, err)
	}
	return true
}

// record writes the audit row for an applied push. The push has already
// committed, so a failure here is logged rather than answered.
func (s *Service) record(r *http.Request, clientID string, revision int64, applied spaces.Applied) {
	if s.audit == nil {
		return
	}
	err := s.audit.Record(r.Context(), clientID, "entitlements.set",
		audit.Target{Type: "entitlements", ID: strconv.FormatInt(revision, 10)},
		map[string]any{
			"spaces": applied.Spaces, "people": applied.People,
			"skipped_spaces": applied.SkippedSpaces, "skipped_people": applied.SkippedPeople,
		})
	if err != nil {
		s.logger.Error("registre: entitlements audit failed", slog.Any("error", err))
	}
}
