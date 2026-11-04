package account

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/passkeys"

	"github.com/go-chi/chi/v5"
)

// passkeyView is one passkey as the account page lists it. The credential id
// and public key stay on the server.
type passkeyView struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	Synced     bool       `json:"synced"`
}

// listPasskeys answers the holder's passkeys.
func (s *Service) listPasskeys(w http.ResponseWriter, r *http.Request) {
	if s.self.Passkeys == nil {
		notFound(w)
		return
	}
	rows, err := s.self.Passkeys.List(r.Context(), holder(r.Context()).ID)
	if err != nil {
		serverError(w, err)
		return
	}
	views := make([]passkeyView, 0, len(rows))
	for _, row := range rows {
		views = append(views, passkeyView{
			ID: row.ID, Name: row.Name, CreatedAt: row.CreatedAt, LastUsedAt: row.LastUsedAt,
			Synced: row.Flags&0x10 != 0,
		})
	}
	encode(w, map[string]any{"passkeys": views}, http.StatusOK)
}

// beginPasskey starts adding a passkey. It needs a fresh factor: a session
// alone must not be able to plant a passkey of its own.
func (s *Service) beginPasskey(w http.ResponseWriter, r *http.Request) {
	if s.self.Passkeys == nil {
		notFound(w)
		return
	}
	if !s.fresh(w, r) {
		return
	}
	user := holder(r.Context())
	options, err := s.self.Passkeys.BeginRegistration(r.Context(), passkeys.PurposeRegister, user.ID, user.Email)
	if err != nil {
		serverError(w, err)
		return
	}
	encode(w, options, http.StatusOK)
}

// finishPasskey verifies and stores the new passkey.
func (s *Service) finishPasskey(w http.ResponseWriter, r *http.Request) {
	body, ok := readAnswer(w, r)
	if !ok || s.self.Passkeys == nil {
		badRequest(w)
		return
	}
	user := holder(r.Context())
	registered, err := s.self.Passkeys.FinishRegistration(r.Context(), body.Ceremony, passkeys.PurposeRegister, body.Credential)
	if err == nil && registered.UserID != user.ID {
		err = passkeys.ErrInvalid
	}
	if err != nil {
		encode(w, map[string]string{"error": "passkey"}, http.StatusBadRequest)
		return
	}
	row, err := s.self.Passkeys.Store(r.Context(), nil, registered, body.Name)
	if err != nil {
		encode(w, map[string]string{"error": "passkey"}, http.StatusBadRequest)
		return
	}
	s.audit(r.Context(), user.ID, "account.passkey_added", map[string]any{"passkey_id": row.ID})
	encode(w, passkeyView{ID: row.ID, Name: row.Name, CreatedAt: row.CreatedAt}, http.StatusCreated)
}

// renamePasskey relabels one passkey.
func (s *Service) renamePasskey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if s.self.Passkeys == nil || json.NewDecoder(r.Body).Decode(&body) != nil {
		badRequest(w)
		return
	}
	err := s.self.Passkeys.Rename(r.Context(), holder(r.Context()).ID, chi.URLParam(r, "id"), body.Name)
	if errors.Is(err, passkeys.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	encode(w, map[string]string{"ok": "true"}, http.StatusOK)
}

// removePasskey deletes one passkey, never the last way into the account,
// and only behind a fresh factor.
func (s *Service) removePasskey(w http.ResponseWriter, r *http.Request) {
	if s.self.Passkeys == nil {
		notFound(w)
		return
	}
	if !s.fresh(w, r) {
		return
	}
	user := holder(r.Context())
	id := chi.URLParam(r, "id")
	err := s.self.Passkeys.Remove(r.Context(), user.ID, id)
	switch {
	case errors.Is(err, passkeys.ErrLastFactor):
		conflict(w, "last_factor")
	case errors.Is(err, passkeys.ErrNotFound):
		notFound(w)
	case err != nil:
		serverError(w, err)
	default:
		s.audit(r.Context(), user.ID, "account.passkey_removed", map[string]any{"passkey_id": id})
		encode(w, map[string]string{"ok": "true"}, http.StatusOK)
	}
}
