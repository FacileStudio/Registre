package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/invites"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// now is the clock the handlers read.
func now() time.Time { return time.Now().UTC() }

func operatorEmail(ctx context.Context) string {
	operator, ok := ctx.Value(operatorKey).(schemas.User)
	if !ok {
		return ""
	}
	return operator.Email
}

// keyStatus reports the signing keys' lifecycle positions. Rotation runs on
// its own loop; this endpoint exists so the dashboard can see the state SPEC
// §7 worries about before it becomes an outage.
func (s *Service) keyStatus(w http.ResponseWriter, r *http.Request) {
	states, err := s.keyring.Status(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	encode(w, states, http.StatusOK)
}

// redeemInvite is the one unauthenticated route: a human trades an invitation
// token for a password. The invite service hashes the password exactly once,
// whether the token was real or not, under the same process-wide ceiling as
// every login.
func (s *Service) redeemInvite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	if body.Token == "" || body.Password == "" {
		encode(w, map[string]string{"error": "token and password are required"}, http.StatusBadRequest)
		return
	}
	if err := s.invites.Redeem(r.Context(), body.Token, body.Password); err != nil {
		if errors.Is(err, invites.ErrInvalid) {
			encode(w, map[string]string{"error": "invalid"}, http.StatusUnauthorized)
			return
		}
		serverError(w, err)
		return
	}
	s.logger.Info("registre: an invitation was redeemed")
	encode(w, map[string]string{"ok": "true"}, http.StatusOK)
}
