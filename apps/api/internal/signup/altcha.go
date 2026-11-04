package signup

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	altcha "github.com/altcha-org/altcha-lib-go"
	"gorm.io/gorm/clause"
)

// ChallengeTTL is how long an ALTCHA challenge may be solved and spent.
const ChallengeTTL = 10 * time.Minute

// MaxNumber bounds the proof-of-work search. A browser finds the answer in
// about half a second on average; a script sending a thousand sign-ups pays
// that a thousand times.
const MaxNumber = 100_000

// MinHMACKeyLength is the shortest ALTCHA_HMAC_KEY that turns sign-up on.
const MinHMACKeyLength = 32

// maxProofLength caps the encoded solution before anything decodes it.
const maxProofLength = 2048

// ErrProof is a missing, forged, expired or replayed proof of work.
var ErrProof = errors.New("signup: invalid proof of work")

// Challenge mints a stateless challenge signed under the HMAC key, expiring
// after ChallengeTTL. Nothing is stored until a solution is spent.
func (s *Service) Challenge() (altcha.Challenge, error) {
	expires := s.now().Add(ChallengeTTL)
	challenge, err := altcha.CreateChallenge(altcha.ChallengeOptions{
		Algorithm: altcha.SHA256, HMACKey: s.key, MaxNumber: MaxNumber, Expires: &expires,
	})
	if err != nil {
		return altcha.Challenge{}, fmt.Errorf("signup: mint a challenge: %w", err)
	}
	return challenge, nil
}

// spend verifies a solution with the official library, in constant time,
// then records its challenge so the same solution can never be spent twice.
// The insert is the replay guard: a second spend conflicts on the primary
// key and affects nothing.
func (s *Service) spend(ctx context.Context, proof string) error {
	payload, ok := decodeProof(proof)
	if !ok {
		return ErrProof
	}
	valid, err := altcha.VerifySolutionSafe(payload, s.key, true)
	if err != nil || !valid {
		return ErrProof
	}
	expires, ok := s.expiry(payload)
	if !ok {
		return ErrProof
	}
	row := schemas.AltchaSpent{Challenge: payload.Challenge, ExpiresAt: expires}
	result := s.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		return fmt.Errorf("signup: record the proof: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrProof
	}
	return nil
}

// decodeProof reads the widget's base64 JSON solution.
func decodeProof(proof string) (altcha.Payload, bool) {
	if proof == "" || len(proof) > maxProofLength {
		return altcha.Payload{}, false
	}
	raw, err := base64.StdEncoding.DecodeString(proof)
	if err != nil {
		return altcha.Payload{}, false
	}
	var payload altcha.Payload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Challenge == "" {
		return altcha.Payload{}, false
	}
	return payload, true
}

// expiry insists the signed salt carries an expiry, in the future and no
// further out than this server would ever have issued.
func (s *Service) expiry(payload altcha.Payload) (time.Time, bool) {
	raw := strings.TrimSuffix(altcha.ExtractParams(payload).Get("expires"), "&")
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	expires := time.Unix(seconds, 0).UTC()
	now := s.now()
	if !expires.After(now) || expires.After(now.Add(ChallengeTTL+time.Minute)) {
		return time.Time{}, false
	}
	return expires, true
}
