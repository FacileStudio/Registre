package directory

import (
	"encoding/json"
	"errors"
	"net/http"

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

// WithWriter turns the push on for the one client named writer, which is
// Ardoise's client id. Every applied push is audited inside its own
// transaction by the directory service. An empty writer leaves the route
// answering 404 to everyone.
func (s *Service) WithWriter(writer string) *Service {
	s.writer = writer
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
	_, err := s.spaces.SetEntitlements(r.Context(), app.ClientID, body.Revision, body.Spaces, body.People)
	if s.refused(w, err) {
		return
	}
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
