package recovery

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

type sent struct {
	to      string
	subject string
	html    string
}

type recorder struct {
	mu       sync.Mutex
	messages []sent
}

func (r *recorder) Send(_ context.Context, to, subject, html string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, sent{to: to, subject: subject, html: html})
	return nil
}

func (r *recorder) all() []sent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]sent(nil), r.messages...)
}

type fixture struct {
	svc    *Service
	users  *identity.Service
	mailer *recorder
	db     *gorm.DB
}

func service(t *testing.T) fixture {
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
	verifier := credentials.New(2)
	mailer := &recorder{}
	return fixture{
		svc:    New(db, verifier, mailer, "https://sso.example.com"),
		users:  identity.New(db, verifier),
		mailer: mailer,
		db:     db,
	}
}

func tokenFrom(t *testing.T, html string) string {
	t.Helper()
	_, after, found := strings.Cut(html, "token=")
	if !found {
		t.Fatalf("the mail carries no token: %s", html)
	}
	token, _, _ := strings.Cut(after, `"`)
	if token == "" {
		t.Fatalf("the mail carries an empty token: %s", html)
	}
	return token
}

func createUser(t *testing.T, users *identity.Service, email, password string) schemas.User {
	t.Helper()
	row, err := users.Create(context.Background(), identity.NewUser{Email: email, Password: password})
	if err != nil {
		t.Fatalf("creating the account: %v", err)
	}
	return row
}

func requestFor(t *testing.T, svc *Service, mailer *recorder, email string) string {
	t.Helper()
	if err := svc.Request(context.Background(), email); err != nil {
		t.Fatalf("requesting a reset: %v", err)
	}
	messages := mailer.all()
	if len(messages) != 1 {
		t.Fatalf("sent %d mails, want 1", len(messages))
	}
	return tokenFrom(t, messages[len(messages)-1].html)
}

func TestRequestRecordsAResetAndMailsTheToken(t *testing.T) {
	f := service(t)
	createUser(t, f.users, "human@example.com", "the old password")

	token := requestFor(t, f.svc, f.mailer, "Human@Example.com")
	message := f.mailer.all()[0]
	if message.to != "human@example.com" || message.subject != Subject {
		t.Fatalf("mailed %q with subject %q", message.to, message.subject)
	}
	var row schemas.PasswordReset
	if err := f.db.Where("token_hash = ?", hash(token)).Take(&row).Error; err != nil {
		t.Fatalf("the mailed token matches no reset: %v", err)
	}
	if strings.Contains(message.html, hash(token)) {
		t.Fatal("the mail carries the stored digest rather than the token")
	}
}

func TestRequestSaysAndRecordsNothingForAnUnknownAddress(t *testing.T) {
	f := service(t)

	if err := f.svc.Request(context.Background(), "nobody@example.com"); err != nil {
		t.Fatalf("an unknown address answered %v, want nil", err)
	}
	if messages := f.mailer.all(); len(messages) != 0 {
		t.Fatalf("sent %d mails for an unknown address", len(messages))
	}
	var count int64
	if err := f.db.Model(&schemas.PasswordReset{}).Count(&count).Error; err != nil {
		t.Fatalf("counting resets: %v", err)
	}
	if count != 0 {
		t.Fatalf("recorded %d resets for an unknown address", count)
	}
}

// The bound is deliberately loose. A wall-clock ratio on a shared CI box
// measures the scheduler as much as the code, so a tight assertion here would
// fail on load and teach whoever hits it to delete the test. What it can
// still catch is the regression that matters: an early return on the unknown
// branch, which drops the argon2 cost entirely and makes that path orders of
// magnitude faster rather than a few percent.
func TestAnUnknownAddressCostsWhatAKnownOneCosts(t *testing.T) {
	f := service(t)
	ctx := context.Background()
	createUser(t, f.users, "known@example.com", "the old password")

	known := elapsed(t, func() error { return f.svc.Request(ctx, "known@example.com") })
	unknown := elapsed(t, func() error { return f.svc.Request(ctx, "unknown@example.com") })
	if unknown*5 < known {
		t.Fatalf("the unknown address took %s against %s for a known one", unknown, known)
	}
}

func elapsed(t *testing.T, run func() error) time.Duration {
	t.Helper()
	started := time.Now()
	if err := run(); err != nil {
		t.Fatalf("requesting a reset: %v", err)
	}
	return time.Since(started)
}

func TestRedeemSetsThePasswordAndSpendsTheReset(t *testing.T) {
	f := service(t)
	ctx := context.Background()
	createUser(t, f.users, "reset@example.com", "the old password")

	token := requestFor(t, f.svc, f.mailer, "reset@example.com")
	if err := f.svc.Redeem(ctx, token, "the new password"); err != nil {
		t.Fatalf("redeeming: %v", err)
	}
	if _, err := f.users.Verify(ctx, "reset@example.com", "the new password"); err != nil {
		t.Fatalf("the new password does not verify: %v", err)
	}
	if _, err := f.users.Verify(ctx, "reset@example.com", "the old password"); !errors.Is(err, identity.ErrInvalid) {
		t.Fatal("the old password still verifies")
	}
	var row schemas.PasswordReset
	if err := f.db.Where("token_hash = ?", hash(token)).Take(&row).Error; err != nil {
		t.Fatalf("reading the reset: %v", err)
	}
	if row.UsedAt == nil {
		t.Fatal("the reset was not marked used")
	}
}

func TestATokenIsSingleUse(t *testing.T) {
	f := service(t)
	ctx := context.Background()
	createUser(t, f.users, "once@example.com", "the old password")

	token := requestFor(t, f.svc, f.mailer, "once@example.com")
	if err := f.svc.Redeem(ctx, token, "first new password"); err != nil {
		t.Fatalf("first redemption: %v", err)
	}
	if err := f.svc.Redeem(ctx, token, "second new password"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("the second redemption answered %v, want ErrInvalid", err)
	}
	if _, err := f.users.Verify(ctx, "once@example.com", "second new password"); !errors.Is(err, identity.ErrInvalid) {
		t.Fatal("the refused redemption still changed the password")
	}
}

func TestSpendingOneResetRetiresTheOthers(t *testing.T) {
	f := service(t)
	ctx := context.Background()
	createUser(t, f.users, "twice@example.com", "the old password")

	first := requestFor(t, f.svc, f.mailer, "twice@example.com")
	if err := f.svc.Request(ctx, "twice@example.com"); err != nil {
		t.Fatalf("requesting a second reset: %v", err)
	}
	second := tokenFrom(t, f.mailer.all()[1].html)
	if err := f.svc.Redeem(ctx, second, "the new password"); err != nil {
		t.Fatalf("redeeming the second reset: %v", err)
	}
	if err := f.svc.Redeem(ctx, first, "another password"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("the older reset answered %v, want ErrInvalid", err)
	}
}

func TestAnExpiredTokenIsRefused(t *testing.T) {
	f := service(t)
	ctx := context.Background()
	createUser(t, f.users, "late@example.com", "the old password")

	token := requestFor(t, f.svc, f.mailer, "late@example.com")
	if err := f.db.Model(&schemas.PasswordReset{}).Where("token_hash = ?", hash(token)).
		Update("expires_at", time.Now().UTC().Add(-time.Minute)).Error; err != nil {
		t.Fatalf("expiring the reset: %v", err)
	}
	if err := f.svc.Redeem(ctx, token, "a new password"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("an expired reset answered %v, want ErrInvalid", err)
	}
}

func TestRedeemDoesNotVerifyTheAddress(t *testing.T) {
	f := service(t)
	ctx := context.Background()
	created := createUser(t, f.users, "unverified@example.com", "the old password")
	if err := f.db.Model(&schemas.User{}).Where("id = ?", created.ID).
		Update("email_verified", false).Error; err != nil {
		t.Fatalf("unverifying the address: %v", err)
	}

	token := requestFor(t, f.svc, f.mailer, "unverified@example.com")
	if err := f.svc.Redeem(ctx, token, "the new password"); err != nil {
		t.Fatalf("redeeming: %v", err)
	}
	after, err := f.users.ByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("reading the account: %v", err)
	}
	if after.EmailVerified {
		t.Fatal("redemption marked the address verified")
	}
}

func TestRedeemDoesNotCreateAMissingAccount(t *testing.T) {
	f := service(t)
	ctx := context.Background()
	created := createUser(t, f.users, "gone@example.com", "the old password")

	token := requestFor(t, f.svc, f.mailer, "gone@example.com")
	if err := f.db.Where("id = ?", created.ID).Delete(&schemas.User{}).Error; err != nil {
		t.Fatalf("deleting the account: %v", err)
	}
	if err := f.svc.Redeem(ctx, token, "the new password"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a reset for a deleted account answered %v, want ErrInvalid", err)
	}
	var count int64
	if err := f.db.Model(&schemas.User{}).Count(&count).Error; err != nil {
		t.Fatalf("counting accounts: %v", err)
	}
	if count != 0 {
		t.Fatalf("redemption created %d accounts", count)
	}
}
