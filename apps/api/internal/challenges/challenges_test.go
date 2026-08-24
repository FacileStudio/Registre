package challenges

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// open returns a migrated database and the user every challenge here hangs
// off. login_challenges carries a foreign key to users, so a challenge for an
// invented subject would fail on the constraint rather than on the logic
// under test.
func open(t *testing.T) (*gorm.DB, schemas.User) {
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
	id, err := ids.New()
	if err != nil {
		t.Fatalf("minting a subject: %v", err)
	}
	user := schemas.User{ID: id, Email: id + "@example.test", Name: "Challenge Holder"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("creating the user: %v", err)
	}
	return db, user
}

// at builds a service whose clock is fixed, so expiry is a decision the test
// makes rather than something it waits for.
func at(db *gorm.DB, moment time.Time) *Service {
	return &Service{db: db, now: func() time.Time { return moment }}
}

func TestAChallengeIsClaimedThenConsumedOnce(t *testing.T) {
	db, user := open(t)
	ctx := context.Background()
	service := New(db)

	token, err := service.Issue(ctx, user.ID, "auth-request-1")
	if err != nil {
		t.Fatalf("issuing: %v", err)
	}
	row, err := service.Claim(ctx, token)
	if err != nil {
		t.Fatalf("claiming: %v", err)
	}
	if row.UserID != user.ID || row.AuthRequestID != "auth-request-1" {
		t.Fatalf("claimed %+v, want the challenge issued for %s", row, user.ID)
	}
	if row.Attempts != 1 {
		t.Errorf("attempts = %d after one claim, want 1", row.Attempts)
	}
	if err := service.Consume(ctx, row.ID); err != nil {
		t.Fatalf("consuming: %v", err)
	}
	if err := service.Consume(ctx, row.ID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("the second consume = %v, want ErrInvalid: two tabs must not both complete one login", err)
	}
	if _, err := service.Claim(ctx, token); !errors.Is(err, ErrInvalid) {
		t.Fatalf("claiming a consumed challenge = %v, want ErrInvalid", err)
	}
}

func TestTheStoredTokenIsOnlyADigest(t *testing.T) {
	db, user := open(t)
	service := New(db)

	token, err := service.Issue(context.Background(), user.ID, "auth-request-1")
	if err != nil {
		t.Fatalf("issuing: %v", err)
	}
	var row schemas.LoginChallenge
	if err := db.Take(&row).Error; err != nil {
		t.Fatalf("reading the row: %v", err)
	}
	if row.TokenHash == token {
		t.Fatal("the plaintext token is in the database: a dump would answer every pending challenge")
	}
	if row.TokenHash != hash(token) {
		t.Fatalf("token_hash = %q, want the SHA-256 of the token", row.TokenHash)
	}
}

func TestAnExpiredChallengeIsRefusedAndPurged(t *testing.T) {
	db, user := open(t)
	ctx := context.Background()
	minted := time.Now().UTC()

	token, err := at(db, minted).Issue(ctx, user.ID, "auth-request-1")
	if err != nil {
		t.Fatalf("issuing: %v", err)
	}
	late := at(db, minted.Add(DefaultTTL+time.Second))
	if _, err := late.Claim(ctx, token); !errors.Is(err, ErrInvalid) {
		t.Fatalf("claiming past the TTL = %v, want ErrInvalid", err)
	}
	if err := late.PurgeExpired(ctx, minted.Add(DefaultTTL+time.Second)); err != nil {
		t.Fatalf("purging: %v", err)
	}
	var left int64
	if err := db.Model(&schemas.LoginChallenge{}).Count(&left).Error; err != nil {
		t.Fatalf("counting: %v", err)
	}
	if left != 0 {
		t.Errorf("%d challenges survived the purge, want 0", left)
	}
}

func TestTheAttemptBudgetRunsOut(t *testing.T) {
	db, user := open(t)
	ctx := context.Background()
	service := New(db)

	token, err := service.Issue(ctx, user.ID, "auth-request-1")
	if err != nil {
		t.Fatalf("issuing: %v", err)
	}
	for i := range MaxAttempts {
		if _, err := service.Claim(ctx, token); err != nil {
			t.Fatalf("claim %d of the budget: %v", i+1, err)
		}
	}
	if _, err := service.Claim(ctx, token); !errors.Is(err, ErrInvalid) {
		t.Fatalf("claim %d = %v, want ErrInvalid: the budget is the whole defence on six digits",
			MaxAttempts+1, err)
	}
}

func TestAnUnknownTokenIsRefused(t *testing.T) {
	db, _ := open(t)
	service := New(db)

	for _, token := range []string{"", "not-a-token"} {
		if _, err := service.Claim(context.Background(), token); !errors.Is(err, ErrInvalid) {
			t.Errorf("claiming %q = %v, want ErrInvalid", token, err)
		}
	}
}
