package account

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// changePassword replaces the holder's password once they have proven the
// current one.
//
// Proving it is not ceremony: the cookie says a browser is logged in, not that
// the person at the keyboard is the owner, and a password that a walk-up
// session can change is a password an attacker uses to lock the owner out.
// The check runs through identity.Verify, so a wrong current password costs
// exactly what a right one costs. An account with two-step sign-in also
// gives a live code or a recovery code: the change ends every session and
// opens a new one, and that must not cost less than the sign-in it replaces.
func (s *Service) changePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		Code            string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	user := holder(r.Context())
	if !s.provesPassword(r.Context(), user, body.CurrentPassword) {
		refuse(w)
		return
	}
	if user.TOTPEnrolled() && !s.provesFactor(r.Context(), user, body.Code) {
		refuse(w)
		return
	}
	if credentials.CheckLength(body.NewPassword) != nil {
		encode(w, map[string]any{
			"error": "the new password is too short", "minimum": MinPasswordLength,
		}, http.StatusBadRequest)
		return
	}
	if credentials.CheckNew(r.Context(), s.self.Pwned, body.NewPassword) != nil {
		encode(w, map[string]string{"error": "password_breached"}, http.StatusBadRequest)
		return
	}
	if err := s.storePassword(r.Context(), user.ID, body.NewPassword); err != nil {
		serverError(w, err)
		return
	}
	if err := s.resealSessions(w, r, user); err != nil {
		serverError(w, err)
		return
	}
	s.logger.Info("registre: an account changed its password", slog.String("user_id", user.ID))
	encode(w, map[string]string{"ok": "true"}, http.StatusOK)
}

// resealSessions ends every session the account holds and opens a fresh one
// for the browser that asked, so the caller stays signed in here and nothing
// else does.
//
// Changing a password is what a person does the moment they think somebody
// else has their account, and until this existed it was a no-op against
// exactly that: the stolen cookie, the live provider sessions and the refresh
// grants all kept working for their full lifetime. The new password only
// governed the next login, which is the login the attacker no longer needs.
//
// The apps those sessions reached are told through back-channel logout, for
// the same reason disabling a user tells them: an app holding its own cookie
// learns a session ended only if registre says so.
func (s *Service) resealSessions(w http.ResponseWriter, r *http.Request, user schemas.User) error {
	reached, err := s.users.EndSessions(r.Context(), user.ID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("account: end the sessions: %w", err)
	}
	if s.notifier != nil && len(reached) > 0 {
		if err := s.notifier.Fire(r.Context(), user.ID, reached); err != nil {
			s.logger.Warn("registre: back-channel logout incomplete after a password change",
				slog.String("user_id", user.ID), slog.Any("error", err))
		}
	}
	return s.reissue(w, r, user)
}

// reissue opens a new session for the caller and replaces their cookie.
//
// A failure here is not fatal to the password change, which has already
// happened and is the thing the caller asked for. It leaves them signed out of
// the account pages, which is the safe direction and is indistinguishable from
// their session having simply ended.
func (s *Service) reissue(w http.ResponseWriter, r *http.Request, user schemas.User) error {
	sessionID, err := s.users.StartSession(r.Context(), user.ID, "", time.Now().UTC())
	if err != nil {
		return fmt.Errorf("account: open the replacement session: %w", err)
	}
	token, err := s.sessions.Issue(r.Context(), sessionID)
	if err != nil {
		s.cookies.Clear(w)
		s.logger.Warn("registre: could not reissue the account cookie",
			slog.String("user_id", user.ID), slog.Any("error", err))
		return nil
	}
	s.cookies.Write(w, token)
	return nil
}

// storePassword hashes a new password under the process-wide argon2 ceiling
// and writes the digest.
//
// It names the one column it owns rather than saving the struct, because a
// full save of a stale row writes back every column the request did not read.
// That default is what once revived a revoked service account with its old
// secret on the next boot, and it is why clients.Upsert names its columns too.
func (s *Service) storePassword(ctx context.Context, userID, password string) error {
	hash, err := s.passwords.Hash(ctx, password)
	if err != nil {
		return fmt.Errorf("account: hash the password: %w", err)
	}
	result := s.db.WithContext(ctx).Model(&schemas.User{}).
		Where("id = ?", userID).
		Update("password_hash", hash)
	if result.Error != nil {
		return fmt.Errorf("account: store the password: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("account: no such user %q", userID)
	}
	return nil
}
