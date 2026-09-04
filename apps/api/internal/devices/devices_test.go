package devices

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// clock is a movable now, so a test can walk a flow past its expiry without
// sleeping through ten minutes of it.
type clock struct{ at time.Time }

func (c *clock) now() time.Time { return c.at }

// flows opens a test database and returns the service over it, plus the clock
// driving it and the handle for reading rows back raw.
//
// Rows are read raw on purpose in the storage tests below. Asserting through
// the same methods that wrote them would prove the service agrees with itself,
// which is exactly the failure mode this repository has already shipped once.
func flows(t *testing.T) (*Service, *clock, *gorm.DB) {
	t.Helper()
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil {
		t.Fatalf("opening the test database: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	ticking := &clock{at: time.Now().UTC()}
	return New(db).WithClock(ticking.now), ticking, db
}

// human writes a user row, because approving a flow names one.
func human(t *testing.T, db *gorm.DB, id string) string {
	t.Helper()
	row := schemas.User{
		ID: id, Email: id + "@facile.studio", PasswordHash: "x",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("creating the user: %v", err)
	}
	return id
}

// startParams holds the parameters for the test helper.
type startParams struct {
	ClientID, DeviceCode, UserCode string
	Expires                        time.Time
}

// start records one flow with the given device and user codes.
func start(t *testing.T, service *Service, p startParams) {
	t.Helper()
	err := service.Store(context.Background(), devices.StoreParams{
		ClientID:   p.ClientID,
		DeviceCode: p.DeviceCode,
		UserCode:   p.UserCode,
		ExpiresAt:  p.Expires,
		Scopes:     []string{"openid"},
	})
	if err != nil {
		t.Fatalf("storing the flow: %v", err)
	}
}

// TestTheDeviceCodeIsStoredAsADigestAndNeverAsItself is the core storage
// assertion: the device code is a bearer credential the CLI presents at the
// token endpoint, so the table must be useless to whoever reads it.
//
// It fails without the hashing in Store — write the plaintext into
// device_code_hash and the first check goes red, then the third, because
// nothing would find the row by digest either.
func TestTheDeviceCodeIsStoredAsADigestAndNeverAsItself(t *testing.T) {
	service, clock, db := flows(t)
	const deviceCode = "8Wg2-not-a-real-device-code"
	start(t, service, startParams{ClientID: "facile-cli", DeviceCode: deviceCode, UserCode: "bcdf-ghjk", Expires: clock.at.Add(Lifetime)})

	var row schemas.DeviceCode
	if err := db.Take(&row).Error; err != nil {
		t.Fatalf("reading the row back: %v", err)
	}
	digest := sha256.Sum256([]byte(deviceCode))
	switch {
	case row.DeviceCodeHash == deviceCode:
		t.Fatal("the device code is stored in the clear; a dump of this table is a list of live credentials")
	case row.DeviceCodeHash != hex.EncodeToString(digest[:]):
		t.Fatalf("device_code_hash = %q, want the SHA-256 of the code", row.DeviceCodeHash)
	case row.UserCode != "BCDFGHJK":
		t.Fatalf("user_code = %q, want the normalised BCDFGHJK", row.UserCode)
	}

	grant, err := service.Poll(context.Background(), "facile-cli", deviceCode)
	if err != nil {
		t.Fatalf("polling a flow that was just stored: %v", err)
	}
	if grant.ClientID != "facile-cli" {
		t.Fatalf("the poll found client %q, want facile-cli", grant.ClientID)
	}
}

// TestAnApprovedFlowIsRedeemableExactlyOnce is the replay guard. zitadel/oidc
// never tells the storage that a token was issued, so if the flow were not
// spent here a captured device code would mint a second, independent set of
// tokens for the rest of its life.
func TestAnApprovedFlowIsRedeemableExactlyOnce(t *testing.T) {
	service, clock, db := flows(t)
	subject := human(t, db, "user-1")
	start(t, service, startParams{ClientID: "facile-cli", DeviceCode: "device-1", UserCode: "BCDF-GHJK", Expires: clock.at.Add(Lifetime)}})

	approval := Approval{UserID: subject, AMR: []string{"pwd"}, AuthTime: clock.at}
	if err := service.Approve(context.Background(), "bcdf ghjk", approval); err != nil {
		t.Fatalf("approving: %v", err)
	}

	first, err := service.Poll(context.Background(), "facile-cli", "device-1")
	if err != nil {
		t.Fatalf("the first poll: %v", err)
	}
	if !first.Approved || first.Subject != subject {
		t.Fatalf("the first poll = %+v, want approved for %s", first, subject)
	}
	if first.AuthTime.IsZero() || len(first.AMR) == 0 {
		t.Errorf("the grant carries no authentication evidence: %+v", first)
	}

	second, err := service.Poll(context.Background(), "facile-cli", "device-1")
	if err != nil {
		t.Fatalf("the second poll: %v", err)
	}
	if second.Approved || !second.Denied {
		t.Fatalf("the second poll = %+v, want denied: a spent device code is replayable", second)
	}
}

// TestAnUnansweredFlowIsPendingAndAnExpiredOneIsNotDenied pins the difference
// the CLI's error message depends on. Both booleans false plus an Expires in
// the past is what makes zitadel answer expired_token; answering denied
// instead would tell a human they were refused when they were merely slow.
func TestAnUnansweredFlowIsPendingAndAnExpiredOneIsNotDenied(t *testing.T) {
	service, clock, _ := flows(t)
	start(t, service, startParams{ClientID: "facile-cli", DeviceCode: "device-1", UserCode: "BCDFGHJK", Expires: clock.at.Add(Lifetime)}))

	pending, err := service.Poll(context.Background(), "facile-cli", "device-1")
	if err != nil {
		t.Fatalf("polling: %v", err)
	}
	if pending.Approved || pending.Denied {
		t.Fatalf("an unanswered flow polls as %+v, want neither approved nor denied", pending)
	}

	clock.at = clock.at.Add(Lifetime + time.Minute)
	expired, err := service.Poll(context.Background(), "facile-cli", "device-1")
	if err != nil {
		t.Fatalf("polling an expired flow: %v", err)
	}
	if expired.Approved || expired.Denied {
		t.Fatalf("an expired flow polls as %+v, want neither", expired)
	}
	if !expired.Expires.Before(clock.at) {
		t.Errorf("the grant's Expires is %s, not in the past, so nothing will answer expired_token", expired.Expires)
	}
}

// TestApprovingAnExpiredFlowIsRefused closes the other half of the same door:
// the answer is written under a condition the database evaluates, so a human
// who leaves the page open past the expiry cannot revive a dead flow.
func TestApprovingAnExpiredFlowIsRefused(t *testing.T) {
	service, clock, db := flows(t)
	subject := human(t, db, "user-1")
	start(t, service, startParams{ClientID: "facile-cli", DeviceCode: "device-1", UserCode: "BCDFGHJK", Expires: clock.at.Add(Lifetime)}))
	clock.at = clock.at.Add(Lifetime + time.Second)

	err := service.Approve(context.Background(), "BCDFGHJK", Approval{UserID: subject, AuthTime: clock.at})
	if err != ErrUnknownCode {
		t.Fatalf("approving an expired flow = %v, want ErrUnknownCode", err)
	}
}

// TestADeniedFlowStaysDenied checks that saying no is recorded rather than
// leaving the flow to time out, so the CLI stops polling immediately.
func TestADeniedFlowStaysDenied(t *testing.T) {
	service, clock, _ := flows(t)
	start(t, service, startParams{ClientID: "facile-cli", DeviceCode: "device-1", UserCode: "BCDFGHJK", Expires: clock.at.Add(Lifetime)}))

	if err := service.Deny(context.Background(), "BCDF-GHJK"); err != nil {
		t.Fatalf("denying: %v", err)
	}
	grant, err := service.Poll(context.Background(), "facile-cli", "device-1")
	if err != nil {
		t.Fatalf("polling: %v", err)
	}
	if !grant.Denied || grant.Approved {
		t.Fatalf("a denied flow polls as %+v, want denied", grant)
	}
	if _, err := service.Pending(context.Background(), "BCDFGHJK"); err != ErrUnknownCode {
		t.Errorf("the verification page can still see an answered flow: %v", err)
	}
}

// TestAFlowBelongsToTheClientThatStartedIt is the binding RFC 8628 §3.4
// requires. Without the client id in the lookup, any client holding a captured
// device code could redeem another client's flow and receive a token minted
// with its own audience.
func TestAFlowBelongsToTheClientThatStartedIt(t *testing.T) {
	service, clock, db := flows(t)
	subject := human(t, db, "user-1")
	start(t, service, startParams{ClientID: "facile-cli", DeviceCode: "device-1", UserCode: "BCDFGHJK", Expires: clock.at.Add(Lifetime)}))
	if err := service.Approve(context.Background(), "BCDFGHJK", Approval{UserID: subject, AuthTime: clock.at}); err != nil {
		t.Fatalf("approving: %v", err)
	}

	if _, err := service.Poll(context.Background(), "some-other-app", "device-1"); err != ErrUnknownCode {
		t.Fatalf("another client redeemed the flow: %v", err)
	}
}

// TestStoringAFlowPurgesTheExpiredOnes is zitadel/oidc's own requirement on
// this interface, and it is not housekeeping: user codes are 34.6 bits, so the
// collision probability the design accepts is a function of how many rows are
// live. A table that only grows makes that number wrong.
func TestStoringAFlowPurgesTheExpiredOnes(t *testing.T) {
	service, clock, db := flows(t)
	start(t, service, startParams{ClientID: "facile-cli", DeviceCode: "device-old", UserCode: "BCDFGHJK", Expires: clock.at.Add(Lifetime)}))

	clock.at = clock.at.Add(Lifetime + purgeAfter + time.Minute)
	start(t, service, startParams{ClientID: "facile-cli", DeviceCode: "device-new", UserCode: "KLMNPQRS", Expires: clock.at.Add(Lifetime)}))

	var rows []schemas.DeviceCode
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("listing the flows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("%d rows survive, want 1: expired flows are not purged", len(rows))
	}
	if rows[0].UserCode != "KLMNPQRS" {
		t.Fatalf("the surviving row is %q, want the fresh one", rows[0].UserCode)
	}
}

// TestACollidingUserCodeIsRefusedByName checks the signal zitadel documents
// for this case. Two live flows sharing one user code would let a human
// approve one device and hand the tokens to another.
func TestACollidingUserCodeIsRefusedByName(t *testing.T) {
	service, clock, _ := flows(t)
	start(t, service, startParams{ClientID: "facile-cli", DeviceCode: "device-1", UserCode: "BCDFGHJK", Expires: clock.at.Add(Lifetime)}))

	err := service.Store(
		context.Background(), devices.StoreParams{
			ClientID:   "facile-cli",
			DeviceCode: "device-2",
			UserCode:   "bcdf-ghjk",
			ExpiresAt:  clock.at.Add(Lifetime),
			Scopes:     nil,
		},
	)
	if err != ErrDuplicateUserCode {
		t.Fatalf("a colliding user code = %v, want ErrDuplicateUserCode", err)
	}
}

// TestNormaliseAcceptsWhatSomebodyActuallyTypes covers the shapes a human
// produces reading eight characters off one screen into another.
func TestNormaliseAcceptsWhatSomebodyActuallyTypes(t *testing.T) {
	for _, typed := range []string{"BCDF-GHJK", "bcdf-ghjk", "bcdf ghjk", " BCDFGHJK ", "bcdf.ghjk"} {
		if got := Normalise(typed); got != "BCDFGHJK" {
			t.Errorf("Normalise(%q) = %q, want BCDFGHJK", typed, got)
		}
	}
	if got := Normalise("'; DROP TABLE device_codes; --"); got != "DRPTBLDVCCDS" {
		t.Errorf("Normalise kept something outside the alphabet: %q", got)
	}
}

// TestTheUserCodeAlphabetIsTwentyUnmistakableCharacters pins the decision the
// package doc explains, because the alphabet is the entropy budget: change its
// size and the 34.6 bits written down everywhere else quietly stops being
// true.
//
// The two exclusions are what make the code typable. No vowels, so no eight
// characters of crypto/rand can spell a word, including the ones nobody wants
// printed by their own identity provider. And no digits at all, which is what
// removes every lookalike pair at once: 0/O, 1/I and 5/S only exist as
// confusions when both halves are in the set.
func TestTheUserCodeAlphabetIsTwentyUnmistakableCharacters(t *testing.T) {
	if len(Alphabet) != 20 || CodeLength != 8 {
		t.Fatalf("the user code is %d characters from an alphabet of %d, want 8 from 20", CodeLength, len(Alphabet))
	}
	seen := map[rune]bool{}
	for _, char := range Alphabet {
		switch {
		case seen[char]:
			t.Errorf("the alphabet repeats %q, so it is smaller than it looks", char)
		case char < 'A' || char > 'Z':
			t.Errorf("the alphabet contains %q, which is not an upper-case letter", char)
		case indexOf("AEIOU", char) >= 0:
			t.Errorf("the alphabet contains the vowel %q, so a code can spell a word", char)
		}
		seen[char] = true
	}
}

// indexOf is strings.IndexRune without the import, kept local so the test
// reads as one thought.
func indexOf(set string, want rune) int {
	for i, char := range set {
		if char == want {
			return i
		}
	}
	return -1
}
