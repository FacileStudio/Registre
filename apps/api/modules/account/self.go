package account

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FacileStudio/Registre/apps/api/internal/gdpr"
	"github.com/FacileStudio/Registre/apps/api/internal/i18n"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// maxNameLength caps the display name, as sign-up does.
const maxNameLength = 100

// updateProfile changes the holder's name and language. The language is
// what every Registre page and mail uses and what apps receive as locale.
func (s *Service) updateProfile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string `json:"name"`
		Locale string `json:"locale"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		badRequest(w)
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || utf8.RuneCountInString(name) > maxNameLength || !i18n.Supported(body.Locale) {
		encode(w, map[string]string{"error": "invalid profile"}, http.StatusBadRequest)
		return
	}
	user := holder(r.Context())
	err := s.db.WithContext(r.Context()).Model(&schemas.User{}).Where("id = ?", user.ID).
		Updates(map[string]any{"name": name, "locale": body.Locale, "updated_at": time.Now().UTC()}).Error
	if err != nil {
		serverError(w, err)
		return
	}
	encode(w, map[string]string{"name": name, "locale": body.Locale}, http.StatusOK)
}

// acceptConsent records the holder's acceptance of the current version of
// every required document. The login that was waiting on it can then finish.
func (s *Service) acceptConsent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Accept bool `json:"accept_terms"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || !body.Accept || s.self.Consents == nil {
		badRequest(w)
		return
	}
	user := holder(r.Context())
	if err := s.self.Consents.Record(r.Context(), nil, user.ID, user.Locale, clientIP(r)); err != nil {
		serverError(w, err)
		return
	}
	s.audit(r.Context(), user.ID, "account.consent", nil)
	encode(w, map[string]string{"ok": "true"}, http.StatusOK)
}

// export answers the holder's Registre data as a JSON download.
func (s *Service) export(w http.ResponseWriter, r *http.Request) {
	if s.self.Rights == nil {
		notFound(w)
		return
	}
	data, err := s.self.Rights.Export(r.Context(), holder(r.Context()))
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="facile-account.json"`)
	w.Header().Set("Cache-Control", "no-store")
	encode(w, data, http.StatusOK)
}

// requestDeletion schedules the holder's account for deletion after the
// grace period. It needs a fresh factor and the account's own address typed
// back as confirmation.
func (s *Service) requestDeletion(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ConfirmEmail string `json:"confirm_email"`
	}
	if s.self.Rights == nil || json.NewDecoder(r.Body).Decode(&body) != nil {
		badRequest(w)
		return
	}
	if !s.fresh(w, r) {
		return
	}
	user := holder(r.Context())
	if !strings.EqualFold(strings.TrimSpace(body.ConfirmEmail), user.Email) {
		encode(w, map[string]string{"error": "confirmation"}, http.StatusBadRequest)
		return
	}
	at, err := s.self.Rights.Request(r.Context(), user)
	var sole *gdpr.SoleOwnerError
	if errors.As(err, &sole) {
		encode(w, map[string]any{"error": "sole_owner", "spaces": sole.Spaces}, http.StatusConflict)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	encode(w, map[string]any{"deletion_scheduled_for": at}, http.StatusAccepted)
}

// cancelDeletion withdraws a pending deletion.
func (s *Service) cancelDeletion(w http.ResponseWriter, r *http.Request) {
	if s.self.Rights == nil {
		notFound(w)
		return
	}
	if err := s.self.Rights.Cancel(r.Context(), holder(r.Context()).ID); err != nil {
		serverError(w, err)
		return
	}
	encode(w, map[string]string{"ok": "true"}, http.StatusOK)
}

// clientIP is the caller's address without its port.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
