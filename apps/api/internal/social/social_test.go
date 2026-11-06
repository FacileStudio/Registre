package social

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

type stubProvider struct{}

func (stubProvider) Name() string { return "discord" }
func (stubProvider) AuthURL(state, challenge, redirectURI string) string {
	return "https://provider.test/auth?state=" + state
}
func (stubProvider) Exchange(context.Context, string, string, string) (Profile, error) {
	return Profile{}, nil
}

func newService(t *testing.T, signupOpen bool) (*Service, *gorm.DB) {
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
	return New(db, []Provider{stubProvider{}}, Config{Issuer: "https://sso.test", SignupOpen: signupOpen}), db
}

func addUser(t *testing.T, db *gorm.DB, email, passwordHash string) schemas.User {
	t.Helper()
	id, err := ids.New()
	if err != nil {
		t.Fatal(err)
	}
	user := schemas.User{ID: id, Email: email, Name: "Existing", PasswordHash: passwordHash, EmailVerified: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("creating %s: %v", email, err)
	}
	return user
}

func profile(subject, email string, verified bool) Profile {
	return Profile{Subject: subject, Email: email, EmailVerified: verified, Name: "Yann"}
}

func TestALinkedIdentitySignsItsUserIn(t *testing.T) {
	svc, db := newService(t, false)
	user := addUser(t, db, "yann@facile.studio", "hash")
	db.Create(&schemas.SocialIdentity{Provider: "discord", Subject: "1", UserID: user.ID})

	got, err := svc.Resolve(context.Background(), "discord", profile("1", "other@else.where", false))
	if err != nil || got.ID != user.ID {
		t.Fatalf("Resolve = %v, %v; want the linked user", got.ID, err)
	}
}

func TestAVerifiedEmailLinksTheExistingAccountAndAnUnverifiedOneDoesNot(t *testing.T) {
	svc, db := newService(t, false)
	user := addUser(t, db, "yann@facile.studio", "hash")

	if _, err := svc.Resolve(context.Background(), "discord", profile("2", "Yann@Facile.Studio", false)); !errors.Is(err, ErrUnverifiedEmail) {
		t.Fatalf("an unverified email must not match an account, got %v", err)
	}
	got, err := svc.Resolve(context.Background(), "discord", profile("2", "Yann@Facile.Studio", true))
	if err != nil || got.ID != user.ID {
		t.Fatalf("a verified email must link the account, got %v, %v", got.ID, err)
	}
	var links int64
	db.Model(&schemas.SocialIdentity{}).Where("user_id = ?", user.ID).Count(&links)
	if links != 1 {
		t.Fatalf("links = %d, want 1", links)
	}
}

func TestAnUnknownEmailCreatesAnAccountOnlyWhileSignupIsOpen(t *testing.T) {
	closed, _ := newService(t, false)
	if _, err := closed.Resolve(context.Background(), "discord", profile("3", "new@facile.studio", true)); !errors.Is(err, ErrSignupClosed) {
		t.Fatalf("closed sign-up must refuse, got %v", err)
	}
	open, db := newService(t, true)
	got, err := open.Resolve(context.Background(), "discord", profile("3", "new@facile.studio", true))
	if err != nil {
		t.Fatalf("open sign-up must create the account: %v", err)
	}
	if got.PasswordHash != "" || !got.EmailVerified || got.Email != "new@facile.studio" {
		t.Fatalf("created user = %+v, want verified and passwordless", got)
	}
	again, err := open.Resolve(context.Background(), "discord", profile("3", "changed@facile.studio", true))
	if err != nil || again.ID != got.ID {
		t.Fatalf("the second sign-in must find the same user, got %v, %v", again.ID, err)
	}
	var users int64
	db.Model(&schemas.User{}).Count(&users)
	if users != 1 {
		t.Fatalf("users = %d, want 1", users)
	}
}

func TestADisabledUserIsRefusedOnEveryPath(t *testing.T) {
	svc, db := newService(t, true)
	user := addUser(t, db, "gone@facile.studio", "hash")
	other := addUser(t, db, "gone2@facile.studio", "hash")
	now := time.Now()
	db.Model(&user).Update("disabled_at", now)
	db.Model(&other).Update("disabled_at", now)

	if _, err := svc.Resolve(context.Background(), "discord", profile("4", "gone@facile.studio", true)); !errors.Is(err, ErrDisabled) {
		t.Fatalf("by email: got %v, want ErrDisabled", err)
	}
	db.Create(&schemas.SocialIdentity{Provider: "discord", Subject: "5", UserID: other.ID})
	if _, err := svc.Resolve(context.Background(), "discord", profile("5", "gone2@facile.studio", true)); !errors.Is(err, ErrDisabled) {
		t.Fatalf("by identity: got %v, want ErrDisabled", err)
	}
}

func TestAStateIsSpentOnceAndOnlyForTheBrowserHoldingIt(t *testing.T) {
	svc, db := newService(t, false)
	started, err := svc.Begin(context.Background(), "discord", "flow-1", "")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	var stored schemas.SocialState
	if err := db.Take(&stored).Error; err != nil || stored.StateHash == started.State {
		t.Fatalf("the state must be stored hashed, got %+v, %v", stored, err)
	}
	if _, err := svc.Take(context.Background(), started.State, "someone else's cookie"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("a state the browser does not hold must be refused, got %v", err)
	}
	row, err := svc.Take(context.Background(), started.State, started.State)
	if err != nil || row.AuthRequestID != "flow-1" || row.Verifier == "" {
		t.Fatalf("Take = %+v, %v", row, err)
	}
	if _, err := svc.Take(context.Background(), started.State, started.State); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("a spent state must be refused, got %v", err)
	}
}

func TestAnExpiredStateIsRefused(t *testing.T) {
	svc, _ := newService(t, false)
	started, err := svc.Begin(context.Background(), "discord", "flow-1", "")
	if err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return time.Now().Add(StateTTL + time.Minute) }
	if _, err := svc.Take(context.Background(), started.State, started.State); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("an expired state must be refused, got %v", err)
	}
}

func TestUnlinkRefusesToLeaveNoWayToSignIn(t *testing.T) {
	svc, db := newService(t, false)
	ctx := context.Background()
	social := addUser(t, db, "social@facile.studio", "")
	db.Create(&schemas.SocialIdentity{Provider: "discord", Subject: "6", UserID: social.ID})
	if err := svc.Unlink(ctx, social.ID, "discord"); !errors.Is(err, ErrLastFactor) {
		t.Fatalf("a passwordless user's last identity must stay, got %v", err)
	}

	keyed := addUser(t, db, "keyed@facile.studio", "")
	db.Create(&schemas.SocialIdentity{Provider: "discord", Subject: "7", UserID: keyed.ID})
	db.Create(&schemas.Passkey{ID: "pk", UserID: keyed.ID, CredentialID: []byte("c"), PublicKey: []byte("k")})
	if err := svc.Unlink(ctx, keyed.ID, "discord"); err != nil {
		t.Fatalf("a user with a passkey may unlink, got %v", err)
	}

	pass := addUser(t, db, "pass@facile.studio", "hash")
	db.Create(&schemas.SocialIdentity{Provider: "discord", Subject: "8", UserID: pass.ID})
	if err := svc.Unlink(ctx, pass.ID, "discord"); err != nil {
		t.Fatalf("a user with a password may unlink, got %v", err)
	}
	if err := svc.Unlink(ctx, pass.ID, "discord"); !errors.Is(err, ErrNotLinked) {
		t.Fatalf("unlinking nothing must say so, got %v", err)
	}
}

func TestLinkingAnAccountAnotherUserHoldsIsRefused(t *testing.T) {
	svc, db := newService(t, false)
	first := addUser(t, db, "a@facile.studio", "hash")
	second := addUser(t, db, "b@facile.studio", "hash")
	if err := svc.LinkTo(context.Background(), first.ID, "discord", profile("9", "", false)); err != nil {
		t.Fatalf("first link: %v", err)
	}
	if err := svc.LinkTo(context.Background(), second.ID, "discord", profile("9", "", false)); !errors.Is(err, ErrTaken) {
		t.Fatalf("second link: got %v, want ErrTaken", err)
	}
	if err := svc.LinkTo(context.Background(), first.ID, "discord", profile("10", "", false)); !errors.Is(err, ErrAlreadyLinked) {
		t.Fatalf("a second discord account for one user: got %v, want ErrAlreadyLinked", err)
	}
}

func TestTheStateCookieIsHostPrefixedWhenSecure(t *testing.T) {
	svc := New(nil, nil, Config{Issuer: "https://sso.test", Secure: true})
	cookie := svc.Cookie("abc")
	if cookie.Name != "__Host-registre-social" || !cookie.Secure || !cookie.HttpOnly || cookie.Path != "/" {
		t.Fatalf("cookie = %+v", cookie)
	}
	request := httptest.NewRequest("GET", "/", nil)
	request.AddCookie(cookie)
	if svc.Held(request) != "abc" {
		t.Fatal("Held did not read back the cookie")
	}
}
