package account

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// enrolmentView is what the enrolment page needs: the QR to scan, the URI to
// follow on the same device, and the seed to type when neither works. All
// three are the same secret, and it crosses this response once.
type enrolmentView struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
	QRPNG  string `json:"qr_png"`
}

// beginTOTP mints an unconfirmed seed for the holder. An account that already
// has a proven factor is refused: replacing a live factor without proving the
// current one would make enrolment the way around enrolment.
func (s *Service) beginTOTP(w http.ResponseWriter, r *http.Request) {
	user := holder(r.Context())
	if user.TOTPEnrolled() {
		conflict(w, "a second factor is already enrolled")
		return
	}
	enrolment, err := s.totp.Begin(r.Context(), user, s.issuer)
	if err != nil {
		serverError(w, err)
		return
	}
	s.logger.Info("registre: an account started TOTP enrolment", slog.String("user_id", user.ID))
	encode(w, enrolmentView{
		Secret: enrolment.Secret, URI: enrolment.URI, QRPNG: enrolment.QRPNG,
	}, http.StatusOK)
}

// confirmTOTP proves a pending enrolment with a live code and answers with the
// recovery codes that come with it, shown exactly once. Nothing stores them in
// plaintext, here or anywhere, so a human who loses this response has to
// regenerate.
func (s *Service) confirmTOTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	user := holder(r.Context())
	codes, err := s.totp.Confirm(r.Context(), user, body.Code)
	if err != nil {
		refuse(w)
		return
	}
	s.logger.Info("registre: an account confirmed a second factor", slog.String("user_id", user.ID))
	encode(w, map[string]any{"recovery_codes": codes}, http.StatusOK)
}

// disableTOTP removes the factor, and wants the current password and a live
// code or recovery code before it does.
//
// Turning a factor off has to cost what turning it on cost. A session alone is
// enough to enrol, but a session alone stripping the factor means a borrowed
// laptop undoes the whole thing, and a second factor that a stolen session can
// remove protects nobody.
func (s *Service) disableTOTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	user := holder(r.Context())
	if !s.proves(r.Context(), user, body.Password, body.Code) {
		refuse(w)
		return
	}
	if err := s.totp.Disable(r.Context(), user.ID); err != nil {
		serverError(w, err)
		return
	}
	s.logger.Info("registre: an account removed its second factor", slog.String("user_id", user.ID))
	encode(w, map[string]string{"ok": "true"}, http.StatusOK)
}

// regenerateRecoveryCodes replaces every code the holder has and answers with
// the new set, shown once.
//
// It wants the password and a factor proof, exactly as disabling does, because
// anything less makes it the way around disabling. With only a password it was
// two requests to strip the second factor from a stolen session: regenerate to
// be handed ten fresh plaintext codes, then present code one as the factor
// proof that disable requires. The gate on disable is only as strong as the
// weakest way to obtain the thing it asks for.
func (s *Service) regenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		badRequest(w)
		return
	}
	user := holder(r.Context())
	if !s.proves(r.Context(), user, body.Password, body.Code) {
		refuse(w)
		return
	}
	if !user.TOTPEnrolled() {
		conflict(w, "no second factor is enrolled")
		return
	}
	codes, err := s.totp.RegenerateRecoveryCodes(r.Context(), user)
	if err != nil {
		serverError(w, err)
		return
	}
	s.logger.Info("registre: an account regenerated its recovery codes", slog.String("user_id", user.ID))
	encode(w, map[string]any{"recovery_codes": codes}, http.StatusOK)
}

// proves reports whether the holder presented both the current password and a
// second-factor proof, which is either a live code or an unspent recovery
// code.
//
// The password is checked first and a wrong one returns before the factor is
// touched. That ordering is deliberate and it reverses what this used to do.
// Checking both unconditionally equalised the timing, but proving a factor
// spends it: Verify burns the time step and RedeemRecoveryCode marks the row
// used. So a wrong password still consumed a recovery code, and anyone holding
// a stolen session could burn all ten by guessing the password ten times,
// leaving the real owner with no way back in after losing their device. That
// is a denial of service against the recovery path, delivered by the check
// meant to protect it.
//
// Nothing is leaked by returning early. The equalisation rule in SPEC §7 exists
// to stop an unauthenticated caller learning whether an account exists; here
// the caller already holds a session for a named account, so there is no
// existence left to disclose. The argon2 hash also dominates both branches by
// orders of magnitude, so the residual timing difference is a DB lookup inside
// a 64 MiB hash.
func (s *Service) proves(ctx context.Context, user schemas.User, password, code string) bool {
	if !s.provesPassword(ctx, user, password) {
		return false
	}
	return s.provesFactor(ctx, user, code)
}

// provesPassword re-verifies the holder's password through identity.Verify, so
// the check runs under the same argon2 ceiling and the same equalised timing
// as a login.
func (s *Service) provesPassword(ctx context.Context, user schemas.User, password string) bool {
	_, err := s.users.Verify(ctx, user.Email, password)
	return err == nil
}

// provesFactor accepts a live TOTP code or an unspent recovery code. Verify
// burns the time step and RedeemRecoveryCode spends the row, so neither proof
// survives to be replayed.
func (s *Service) provesFactor(ctx context.Context, user schemas.User, code string) bool {
	if err := s.totp.Verify(ctx, user, code); err == nil {
		return true
	}
	return s.totp.RedeemRecoveryCode(ctx, user.ID, code) == nil
}
