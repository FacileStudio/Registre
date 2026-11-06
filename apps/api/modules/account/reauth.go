package account

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/audit"
)

// maxCeremonyBody caps a WebAuthn answer.
const maxCeremonyBody = 64 << 10

// ceremonyAnswer is the browser's answer to a passkey ceremony.
type ceremonyAnswer struct {
	Ceremony   string          `json:"ceremony"`
	Credential json.RawMessage `json:"credential"`
	Name       string          `json:"name"`
}

// readAnswer decodes a ceremony answer under the size cap.
func readAnswer(w http.ResponseWriter, r *http.Request) (ceremonyAnswer, bool) {
	var body ceremonyAnswer
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCeremonyBody)).Decode(&body)
	return body, err == nil
}

// fresh answers whether the request's session proved a factor recently, and
// refuses with 401 reauth_required otherwise.
func (s *Service) fresh(w http.ResponseWriter, r *http.Request) bool {
	_, session := accounts.Holder(r.Context())
	if s.sessions.Fresh(session) {
		return true
	}
	encode(w, map[string]string{"error": "reauth_required"}, http.StatusUnauthorized)
	return false
}

// reauthPassword proves the password again, and the second factor too when
// the account has one, so a reauthentication is never weaker than the
// account's own login.
func (s *Service) reauthPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		badRequest(w)
		return
	}
	user := holder(r.Context())
	proven := s.provesPassword(r.Context(), user, body.Password)
	if proven && user.TOTPEnrolled() {
		proven = s.provesFactor(r.Context(), user, body.Code)
	}
	if !proven {
		refuse(w)
		return
	}
	s.markFresh(w, r)
}

// beginReauthPasskey starts a passkey reauthentication.
func (s *Service) beginReauthPasskey(w http.ResponseWriter, r *http.Request) {
	if s.self.Passkeys == nil {
		notFound(w)
		return
	}
	user := holder(r.Context())
	options, err := s.self.Passkeys.BeginReauth(r.Context(), user.ID, user.Email)
	if err != nil {
		refuse(w)
		return
	}
	encode(w, options, http.StatusOK)
}

// finishReauthPasskey verifies the passkey reauthentication.
func (s *Service) finishReauthPasskey(w http.ResponseWriter, r *http.Request) {
	body, ok := readAnswer(w, r)
	if !ok || s.self.Passkeys == nil {
		badRequest(w)
		return
	}
	user := holder(r.Context())
	if err := s.self.Passkeys.FinishReauth(r.Context(), body.Ceremony, user.ID, body.Credential); err != nil {
		refuse(w)
		return
	}
	s.markFresh(w, r)
}

// markFresh stamps the session as freshly reauthenticated.
func (s *Service) markFresh(w http.ResponseWriter, r *http.Request) {
	_, session := accounts.Holder(r.Context())
	if err := s.sessions.MarkFactorProven(r.Context(), session.ID); err != nil {
		serverError(w, err)
		return
	}
	encode(w, map[string]string{"ok": "true"}, http.StatusOK)
}

// audit writes one account audit row, actor and target both the holder.
func (s *Service) audit(ctx context.Context, userID, action string, detail map[string]any) {
	if s.self.Audit == nil {
		return
	}
	if err := s.self.Audit.Record(ctx, userID, action, audit.Target{Type: "user", ID: userID}, detail); err != nil {
		s.logger.Warn("registre: audit write failed", slog.String("action", action), slog.Any("error", err))
	}
}

// notFound answers a route whose service this deployment does not run.
func notFound(w http.ResponseWriter) {
	encode(w, map[string]string{"error": "not found"}, http.StatusNotFound)
}
