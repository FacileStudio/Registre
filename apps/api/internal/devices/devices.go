// Package devices owns the OAuth 2.0 device authorization grant, RFC 8628.
//
// It exists because the loopback flow every suite CLI uses today cannot work
// when the browser is somewhere else. `facile login` on a server opens
// http://127.0.0.1:<port> and tells the human's browser to come back to it —
// but on a Chromebook, 127.0.0.1 is the Chromebook. The redirect lands
// nowhere, the code expires unused, and nothing in the logs says why. The
// device grant deletes the assumption: the CLI shows a short code, the human
// approves it on any device that has a browser, and the CLI polls the token
// endpoint until an answer appears here.
//
// It owns no HTTP. modules/oidc adapts it to the two zitadel/oidc storage
// methods, and modules/device serves the three routes the verification page
// calls. SPEC §5 is the rule and filet enforces it.
//
// # The user code, which is the part that can be guessed
//
// The alphabet is BCDFGHJKLMNPQRSTVWXZ — twenty characters, eight of them per
// code, grouped four and four with a dash for reading. That is 20^8, or
// **34.6 bits**, which is the worked example RFC 8628 §6.1 gives for exactly
// this decision.
//
// The exclusions are all about a human retyping what they read off another
// screen. There are no digits, so 0/O and 1/I/l cannot be confused with
// anything. The vowels are gone, so no code can spell a word — including the
// ones nobody wants printed by their own identity provider. What is left has
// no lookalike pair at any common terminal or browser font.
//
// 34.6 bits is not a lot, and RFC 8628 §5.2 says so: the endpoint that accepts
// a user code MUST be rate limited. Three things defend it, and the entropy is
// the least of them. The code lives ten minutes. It is spent once. And the
// three routes that accept one are limited to five requests a second per
// source address, the same budget the password endpoint gets. One address
// therefore gets at most about 3,000 guesses inside a code's whole life,
// against 2.6 × 10^10 possibilities — with ten flows live at once, roughly one
// chance in 850,000 of hitting any of them.
//
// Approval also requires a signed-in human, so a guesser who does land on a
// live code still has to be somebody before the code means anything.
package devices

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Alphabet, CodeLength and GroupSize are the user code's shape. The package
// doc above carries the reasoning; zitadel/oidc is configured from these
// constants rather than from its own equivalents so that the explanation and
// the codes actually minted cannot drift apart.
const (
	Alphabet   = "BCDFGHJKLMNPQRSTVWXZ"
	CodeLength = 8
	GroupSize  = 4
)

// VerificationPath is where a human goes to answer a device flow. The client
// build serves the page, discovery hands the path to the CLI, and the CLI
// prints it next to the user code, so changing it here changes what every
// terminal in the suite tells people to type into a browser.
const VerificationPath = "/device"

// Lifetime is how long a device flow stays answerable. It bounds three
// windows at once: how long a guesser has against one user code, how long a
// stolen device code is worth stealing, and how long somebody has to walk to
// another computer. Ten minutes is the same bound a browser login gets.
const Lifetime = 10 * time.Minute

// PollInterval is what the CLI is told to wait between polls, in the
// device authorization response. RFC 8628 §3.2's default, and the value
// zitadel's own token handler assumes when it gives a poll four seconds.
const PollInterval = 5 * time.Second

// purgeAfter is how long an expired flow's row survives.
//
// It is not zero, and that is the point: zitadel answers a poll on a flow it
// cannot find with access_denied, and a poll on a flow it can find but that
// has expired with expired_token. Deleting the row the instant it expires
// would turn every honest "you took too long" into "you are not allowed",
// which is the wrong thing to print and the wrong thing to page somebody
// about. An hour is long enough that no CLI is still polling.
const purgeAfter = time.Hour

// ErrUnknownCode is a code that names no answerable flow: never issued,
// already answered, or expired. One error for all three deliberately — the
// endpoints that report it are the ones a guesser would use to tell them
// apart.
var ErrUnknownCode = errors.New("devices: unknown, spent or expired code")

// ErrDuplicateUserCode is a minted user code that collided with a live one.
// The caller is expected to mint another and try again; modules/oidc maps it
// onto op.ErrDuplicateUserCode, which is the name zitadel gives the same fact.
var ErrDuplicateUserCode = errors.New("devices: the user code is already in use")

// Approval is what a human's yes contributes to the token that follows: who
// they are, how they proved it, and when.
type Approval struct {
	UserID   string
	AMR      []string
	AuthTime time.Time
}

// Grant is what one poll of a device flow found. It is the shape
// op.DeviceAuthorizationState wants, expressed without importing the library:
// this package decides what a flow's state is, and modules/oidc translates.
type Grant struct {
	ClientID string
	Scopes   []string
	Expires  time.Time

	// Approved and Denied are never both true. Both false means the human
	// has not answered yet, or the flow expired unanswered — the caller
	// tells those apart by reading Expires, because that is the only way
	// to answer authorization_pending and expired_token differently.
	Approved bool
	Denied   bool

	Subject  string
	AMR      []string
	AuthTime time.Time
}

// Service mints, answers and spends device flows.
type Service struct {
	db  *gorm.DB
	now func() time.Time
}

// New builds the service over db.
func New(db *gorm.DB) *Service {
	return &Service{db: db, now: func() time.Time { return time.Now().UTC() }}
}

// WithClock replaces the service's clock, so a test can move a flow past its
// expiry without sleeping through it.
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

// Store records a freshly minted flow and purges the expired ones on the way
// past.
//
// The purge is here rather than on a ticker because this is the only statement
// that adds a row, so it is the only moment the table can grow, and because
// zitadel/oidc's interface documentation makes purging a requirement rather
// than housekeeping: user codes are low entropy, and the collision probability
// it warns about is a function of how many rows are live. A sweep that runs
// exactly when the count goes up needs no goroutine, no shutdown handling and
// no second place to look when the table is unexpectedly large.
func (s *Service) Store(
	ctx context.Context, clientID, deviceCode, userCode string, expires time.Time, scopes []string,
) error {
	if err := s.purge(ctx); err != nil {
		return err
	}
	row := schemas.DeviceCode{
		DeviceCodeHash: hash(deviceCode),
		UserCode:       Normalise(userCode),
		ClientID:       clientID,
		Scopes:         schemas.StringList(scopes),
		ExpiresAt:      expires,
		CreatedAt:      s.now(),
	}
	result := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		return fmt.Errorf("devices: record the flow: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrDuplicateUserCode
	}
	return nil
}

// Poll answers one attempt at the token endpoint and, when the answer is yes,
// spends the flow in the same breath.
//
// Spending it here is what makes a device code single use. zitadel/oidc calls
// this method once per poll and never tells the storage that a token was
// issued, so if the row survived the successful poll the device code would
// stay redeemable for the rest of its ten minutes — a captured one would mint
// a second, independent set of tokens. An authorization code is burnt at
// exactly the same point in this repository, for exactly this reason.
//
// The cost is that a CLI which loses the response never gets another: its next
// poll reads the flow as denied and the human starts again. That is the right
// side to fail on, and it is a network failure the human can see, rather than
// a replay nobody can.
func (s *Service) Poll(ctx context.Context, clientID, deviceCode string) (Grant, error) {
	var row schemas.DeviceCode
	err := s.db.WithContext(ctx).
		Where("device_code_hash = ? AND client_id = ?", hash(deviceCode), clientID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Grant{}, ErrUnknownCode
	}
	if err != nil {
		return Grant{}, fmt.Errorf("devices: read the flow: %w", err)
	}
	grant := Grant{ClientID: row.ClientID, Scopes: row.Scopes, Expires: row.ExpiresAt}
	switch {
	case row.Denied():
		grant.Denied = true
	case row.Approved() && row.ExpiresAt.After(s.now()):
		return s.spend(ctx, row, grant)
	}
	return grant, nil
}

// spend stamps the flow as redeemed and reports the grant only to whoever won
// that update. A caller that finds the stamp already set is a replay, and a
// replay is refused: zitadel offers no outcome between yes and no, so it is
// told no.
func (s *Service) spend(ctx context.Context, row schemas.DeviceCode, grant Grant) (Grant, error) {
	result := s.db.WithContext(ctx).Model(&schemas.DeviceCode{}).
		Where("device_code_hash = ? AND redeemed_at IS NULL", row.DeviceCodeHash).
		Update("redeemed_at", s.now())
	if result.Error != nil {
		return Grant{}, fmt.Errorf("devices: spend the flow: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		grant.Denied = true
		return grant, nil
	}
	grant.Approved = true
	grant.Subject, grant.AMR, grant.AuthTime = row.Subject(), row.AMR, row.AuthenticatedAt()
	return grant, nil
}

// purge drops flows that expired more than purgeAfter ago.
func (s *Service) purge(ctx context.Context) error {
	cutoff := s.now().Add(-purgeAfter)
	err := s.db.WithContext(ctx).Where("expires_at < ?", cutoff).Delete(&schemas.DeviceCode{}).Error
	if err != nil {
		return fmt.Errorf("devices: purge expired flows: %w", err)
	}
	return nil
}

// Normalise is the one form a user code is stored and looked up in: upper
// case, with everything that is not in the alphabet removed.
//
// It runs on the way in as well as the way out, so a code typed as
// "bcdf ghjk", "BCDF-GHJK" or "bcdf.ghjk" all find the same row. Dropping
// unknown characters rather than rejecting them also means no lookup key can
// carry a wildcard, a quote or a percent sign into whatever reads it next.
func Normalise(userCode string) string {
	var out strings.Builder
	out.Grow(len(userCode))
	for _, char := range strings.ToUpper(userCode) {
		if strings.ContainsRune(Alphabet, char) {
			out.WriteRune(char)
		}
	}
	return out.String()
}

// hash digests a device code for storage and lookup. SHA-256 without a work
// factor is right here for the same reason it is right for an authorization
// code: the value is already 128 bits of crypto/rand, so there is no preimage
// to slow anybody down to, and the hash exists so that a captured database is
// not a list of working credentials.
func hash(deviceCode string) string {
	sum := sha256.Sum256([]byte(deviceCode))
	return hex.EncodeToString(sum[:])
}
