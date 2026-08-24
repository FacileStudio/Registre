// Package totp owns the second factor: enrolling one, proving it, and
// spending a recovery code when the device that held it is gone.
//
// The algorithm comes from pquerna/otp rather than from here. TOTP is forty
// lines of HMAC over a counter and hand-rolling it is defensible, but the
// library also carries the parts that are easy to get quietly wrong: the
// skew window, the constant-time comparison, and the provisioning URI and QR
// an authenticator app expects. Trading one dependency for a hand-rolled
// verifier and a hand-rolled QR encoder is the wrong side of that bargain.
//
// It owns no routes, so it lives under internal/ by the same rule that keeps
// keys and clients there. modules/account and modules/login own its HTTP.
package totp

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/crypto"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
)

// ErrInvalid is every refusal this package makes: a wrong code, a replayed
// one, an account with no factor, a spent recovery code. One error for all of
// them, because distinguishing them for the caller distinguishes them for
// whoever is guessing.
var ErrInvalid = errors.New("totp: invalid code")

// Period and Digits are the RFC 6238 defaults, which is what every
// authenticator app assumes when the provisioning URI omits them.
const (
	Period = 30
	Digits = otp.DigitsSix
)

// Skew is how many time steps either side of now a code is accepted in,
// giving a phone clock thirty seconds of drift in each direction. One step is
// the usual choice: it triples the guessing surface from 10^-6 to 3x10^-6,
// which the attempt budget in modules/login answers, while a tighter window
// refuses honest people whose clocks are merely ordinary.
const Skew = 1

// QRSize is the edge of the enrolment QR in pixels.
const QRSize = 256

// Enrolment is what a human needs to add the account to an authenticator
// app: the seed to type by hand, the URI to follow, and the QR to scan.
type Enrolment struct {
	Secret string
	URI    string
	QRPNG  string
}

// Service enrols, verifies and revokes second factors.
type Service struct {
	db   *gorm.DB
	root []byte
	now  func() time.Time
}

// New builds the service over db, sealing seeds under root.
func New(db *gorm.DB, root []byte) *Service {
	return &Service{db: db, root: root, now: func() time.Time { return time.Now().UTC() }}
}

// Begin mints an unconfirmed seed for user and returns what the enrolment
// page shows. It overwrites any previous unconfirmed seed, so a human who
// abandons the flow and starts again is not stuck with the first one.
//
// It refuses an account that already has a confirmed factor. Replacing a live
// factor without proving the current one would make enrolment itself the way
// around enrolment.
func (s *Service) Begin(ctx context.Context, user schemas.User, issuer string) (Enrolment, error) {
	if user.TOTPEnrolled() {
		return Enrolment{}, fmt.Errorf("totp: %s already has a second factor", user.Email)
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer: issuer, AccountName: user.Email, Period: Period, Digits: Digits,
	})
	if err != nil {
		return Enrolment{}, fmt.Errorf("totp: generate the seed: %w", err)
	}
	sealed, nonce, err := crypto.Seal([]byte(key.Secret()), s.root)
	if err != nil {
		return Enrolment{}, fmt.Errorf("totp: seal the seed: %w", err)
	}
	err = s.db.WithContext(ctx).Model(&schemas.User{}).
		Where("id = ? AND totp_confirmed_at IS NULL", user.ID).
		Updates(map[string]any{"totp_secret": sealed, "totp_nonce": nonce}).Error
	if err != nil {
		return Enrolment{}, fmt.Errorf("totp: store the seed: %w", err)
	}
	qr, err := encodeQR(key)
	if err != nil {
		return Enrolment{}, err
	}
	return Enrolment{Secret: key.Secret(), URI: key.URL(), QRPNG: qr}, nil
}

// Confirm proves a pending enrolment with a live code and returns the
// recovery codes that come with it, shown exactly once.
//
// The factor becomes live and the codes are written in one transaction. A
// confirmed factor with no recovery codes is an account one lost phone away
// from needing an operator, which is the thing phase 3 exists to end.
//
// Confirming deliberately does not burn the time step the way Verify does.
// It used to, and the cost was the whole first-run experience: enrolment and
// the next login fall inside the same thirty seconds, so the code the
// authenticator was still displaying came back refused, and the refusal is
// the same generic one a wrong code gets. The step burn buys nothing here.
// Whoever reaches this call already holds the account session and the seed,
// so replaying the code as a login gains them nothing they cannot already do,
// and enrolment itself is single-use by construction through the
// totp_confirmed_at IS NULL guard below.
func (s *Service) Confirm(ctx context.Context, user schemas.User, code string) ([]string, error) {
	if user.TOTPEnrolled() {
		return nil, fmt.Errorf("totp: %s already has a second factor", user.Email)
	}
	secret, err := s.unseal(user)
	if err != nil {
		return nil, err
	}
	at := s.now()
	if !validate(code, secret, at) {
		return nil, ErrInvalid
	}
	plain, hashes, err := mintRecoveryCodes()
	if err != nil {
		return nil, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&schemas.User{}).
			Where("id = ? AND totp_confirmed_at IS NULL", user.ID).
			Updates(map[string]any{"totp_confirmed_at": at})
		if result.Error != nil {
			return fmt.Errorf("totp: confirm the factor: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrInvalid
		}
		return storeRecoveryCodes(tx, user.ID, hashes, at)
	})
	if err != nil {
		return nil, err
	}
	return plain, nil
}

// Verify checks code against user's confirmed factor and burns the time step
// it belongs to, so the same code cannot be presented twice.
//
// The step is claimed with a conditional update rather than a read followed
// by a write: two requests arriving with the same code in the same window
// would otherwise both read the old step and both succeed, which is the
// replay the column exists to stop.
func (s *Service) Verify(ctx context.Context, user schemas.User, code string) error {
	if !user.TOTPEnrolled() {
		return ErrInvalid
	}
	secret, err := s.unseal(user)
	if err != nil {
		return err
	}
	at := s.now()
	if !validate(code, secret, at) {
		return ErrInvalid
	}
	step := at.Unix() / Period
	result := s.db.WithContext(ctx).Model(&schemas.User{}).
		Where("id = ? AND (totp_last_step IS NULL OR totp_last_step < ?)", user.ID, step).
		Update("totp_last_step", step)
	if result.Error != nil {
		return fmt.Errorf("totp: claim the time step: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrInvalid
	}
	return nil
}

// Disable removes the factor and every recovery code with it, in one
// transaction. Leaving the codes behind would leave a way into an account
// whose owner believes they have turned the factor off.
func (s *Service) Disable(ctx context.Context, userID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&schemas.User{}).Where("id = ?", userID).
			Updates(map[string]any{
				"totp_secret": nil, "totp_nonce": nil,
				"totp_confirmed_at": nil, "totp_last_step": nil,
			}).Error
		if err != nil {
			return fmt.Errorf("totp: clear the factor: %w", err)
		}
		if err := tx.Where("user_id = ?", userID).Delete(&schemas.RecoveryCode{}).Error; err != nil {
			return fmt.Errorf("totp: clear the recovery codes: %w", err)
		}
		return nil
	})
}

// unseal opens the stored seed with the process root key.
func (s *Service) unseal(user schemas.User) (string, error) {
	if len(user.TOTPSecret) == 0 || len(user.TOTPNonce) == 0 {
		return "", ErrInvalid
	}
	secret, err := crypto.Open(user.TOTPSecret, user.TOTPNonce, s.root)
	if err != nil {
		return "", fmt.Errorf("totp: open the seed: %w", err)
	}
	return string(secret), nil
}

// validate checks one code against a seed at a moment, inside the skew
// window. It never returns an error: an unparseable code is a wrong code.
func validate(code, secret string, at time.Time) bool {
	ok, err := totp.ValidateCustom(code, secret, at, totp.ValidateOpts{
		Period: Period, Skew: Skew, Digits: Digits, Algorithm: otp.AlgorithmSHA1,
	})
	return err == nil && ok
}

// encodeQR renders the provisioning URI as a base64 PNG data payload, ready
// for an <img src="data:image/png;base64,...">.
func encodeQR(key *otp.Key) (string, error) {
	image, err := key.Image(QRSize, QRSize)
	if err != nil {
		return "", fmt.Errorf("totp: render the QR: %w", err)
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, image); err != nil {
		return "", fmt.Errorf("totp: encode the QR: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buffer.Bytes()), nil
}
