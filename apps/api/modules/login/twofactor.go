// The two-step half of the login module: the challenge minted when a
// password checks out against an account that owes a second factor, and the
// endpoint that answers it.
//
// It sits beside handlers.go rather than inside it because the file limit is
// 300 lines and one login file carrying both steps was past it.

package login

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/challenges"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/totp"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

// SecondFactor bundles what a two-step login needs: the challenge store that
// carries "the password was right" from one request to the next, the factor
// that answers it, and the account session both paths mint at the end. It is
// a struct rather than four parameters so the builder stays inside the
// parameter ceiling as the account pages grow.
type SecondFactor struct {
	Challenges *challenges.Service
	Codes      *totp.Service
	Accounts   *accounts.Service
	Cookies    accounts.Cookies
}

// jsonSecondFactor answers the second step: a challenge plus either the six
// digits from the authenticator app or one recovery code. It refuses exactly
// as the password endpoint refuses, so a wrong code, a spent challenge and an
// invented one are one answer.
func (s *Service) jsonSecondFactor(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Challenge string `json:"challenge"`
		Code      string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	answer, err := s.answer(r.Context(), w, body.Challenge, body.Code)
	if err != nil {
		s.refuse(w, err)
		return
	}
	encode(w, answer, http.StatusOK)
}

// answer spends a challenge against a code and finishes the login it was
// minted for.
//
// The challenge is claimed before the code is checked, so every guess costs
// an attempt whether or not the rest of the row still holds, and the flow is
// consumed before the session opens, so two tabs racing the same good code
// cannot both complete it.
func (s *Service) answer(
	ctx context.Context, w http.ResponseWriter, token, code string,
) (decision, error) {
	if s.factor.Challenges == nil || s.factor.Codes == nil {
		return decision{}, challenges.ErrInvalid
	}
	row, err := s.factor.Challenges.Claim(ctx, token)
	if err != nil {
		return decision{}, err
	}
	user, err := s.identity.ByID(ctx, row.UserID)
	if err != nil {
		return decision{}, err
	}
	if !user.Enabled() {
		return decision{}, identity.ErrInvalid
	}
	if err := s.checkCode(ctx, user, code); err != nil {
		return decision{}, err
	}
	if err := s.factor.Challenges.Consume(ctx, row.ID); err != nil {
		return decision{}, err
	}
	if row.AuthRequestID == "" {
		redirect, err := s.completeDirect(ctx, w, user.ID)
		if err != nil {
			return decision{}, err
		}
		return decision{Redirect: redirect}, nil
	}
	authReq, err := s.identity.AuthRequestByID(ctx, row.AuthRequestID)
	if err != nil || !authReq.ExpiresAt.After(s.now()) || authReq.Done() {
		return decision{}, errAuthRequestGone
	}
	redirect, err := s.complete(ctx, w, user.ID, authReq)
	if err != nil {
		return decision{}, err
	}
	return decision{Redirect: redirect}, nil
}

// checkCode accepts either a live authenticator code or one recovery code.
// The authenticator is tried first because it is the normal case and costs no
// write when it misses; the recovery table is only touched once the digits
// have already been refused.
func (s *Service) checkCode(ctx context.Context, user schemas.User, code string) error {
	err := s.factor.Codes.Verify(ctx, user, code)
	if err == nil {
		return nil
	}
	if !errors.Is(err, totp.ErrInvalid) {
		return err
	}
	return s.factor.Codes.RedeemRecoveryCode(ctx, user.ID, code)
}

// errNoSecondFactor answers 401 rather than 500, and is logged.
//
// It fails closed: an enrolled account never completes a login on a build
// that cannot check its factor. The status matters because it only fires on a
// correct password, so a 500 here would be the one response that names an
// account as having a second factor.
//
// errNoSecondFactor is an account with a confirmed factor meeting a
// deployment that has no service to check one with. It is an operator's bug
// and answers 500, because the alternative is logging the human in on the
// password alone.
var errNoSecondFactor = errors.New("login: the account has a second factor and this build cannot check one")
