package signup

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/consent"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

const goodPassword = "correct horse battery staple"

// linkFor runs Start for email and answers the mailed token.
func linkFor(t *testing.T, w world, email string) string {
	t.Helper()
	if err := w.svc.Start(context.Background(), Request{Email: email, Proof: solve(t, w.svc), Locale: "de"}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return tokenIn(t, w.inbox.last(t).html)
}

func form(token string) Account {
	return Account{Token: token, Name: "Ada", Locale: "es", Accepted: true, IP: "203.0.113.7", Password: goodPassword}
}

// TestALinkCreatesOneVerifiedAccountWithConsent covers verify, finish,
// single use and the consent rows.
func TestALinkCreatesOneVerifiedAccountWithConsent(t *testing.T) {
	w := newWorld(t, map[string]bool{})
	ctx := context.Background()
	token := linkFor(t, w, "Ada@Example.com")
	pending, err := w.svc.Verify(ctx, token)
	if err != nil || pending.Email != "ada@example.com" || pending.Locale != "de" {
		t.Fatalf("Verify = %+v, %v", pending, err)
	}
	created, err := w.svc.Finish(ctx, form(token))
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	user := created.User
	if !user.EmailVerified || user.EmailVerifiedAt == nil || user.Locale != "es" || user.PasswordHash == "" {
		t.Fatalf("created %+v, want a verified es account with a password", user)
	}
	if _, err := w.svc.Finish(ctx, form(token)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("second Finish = %v, want ErrInvalid", err)
	}
	var rows []schemas.Consent
	w.db.Where("user_id = ?", user.ID).Find(&rows)
	if len(rows) != len(consent.Required()) {
		t.Fatalf("%d consent rows, want %d", len(rows), len(consent.Required()))
	}
	for _, row := range rows {
		if row.Version != consent.Versions()[row.Document] || row.IPHash == "" || row.IPHash == "203.0.113.7" {
			t.Fatalf("consent row %+v: wrong version or IP not digested", row)
		}
	}
	owed, _ := consent.New(w.db, []byte("consent-key")).Outdated(ctx, user.ID)
	if len(owed) != 0 {
		t.Fatalf("a new account owes %v", owed)
	}
}

// TestAnExpiredLinkIsRefused.
func TestAnExpiredLinkIsRefused(t *testing.T) {
	w := newWorld(t, map[string]bool{})
	token := linkFor(t, w, "late@example.com")
	w.db.Model(&schemas.SignupChallenge{}).Where("email = ?", "late@example.com").
		Update("expires_at", time.Now().Add(-time.Second))
	if _, err := w.svc.Verify(context.Background(), token); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Verify on an expired link = %v, want ErrInvalid", err)
	}
	if _, err := w.svc.Finish(context.Background(), form(token)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Finish on an expired link = %v, want ErrInvalid", err)
	}
}

// TestPasswordRulesFollowNIST: eight characters, no composition rules,
// breached passwords refused.
func TestPasswordRulesFollowNIST(t *testing.T) {
	w := newWorld(t, map[string]bool{"passwordpassword": true})
	ctx := context.Background()
	cases := map[string]error{
		"seven c":                 credentials.ErrPasswordTooShort,
		"passwordpassword":        ErrBreached,
		string(make([]rune, 129)): credentials.ErrPasswordTooLong,
	}
	for password, want := range cases {
		in := form(linkFor(t, w, fmt.Sprintf("nist%d@example.com", len([]rune(password)))))
		in.Password = password
		if _, err := w.svc.Finish(ctx, in); !errors.Is(err, want) {
			t.Errorf("password %q = %v, want %v", password, err, want)
		}
	}
	in := form(linkFor(t, w, "nist@example.com"))
	in.Password = "eightchr"
	if _, err := w.svc.Finish(ctx, in); err != nil {
		t.Fatalf("an eight-character password = %v, want accepted", err)
	}
}

// TestAnHIBPOutageNeverBlocksSignup.
func TestAnHIBPOutageNeverBlocksSignup(t *testing.T) {
	w := newWorld(t, nil)
	if _, err := w.svc.Finish(context.Background(), form(linkFor(t, w, "offline@example.com"))); err != nil {
		t.Fatalf("Finish with HIBP down = %v, want accepted", err)
	}
}

// TestTheFormIsChecked refuses a missing acceptance, an unknown locale, a
// missing name and a missing password: sign-up never creates an account
// without one.
func TestTheFormIsChecked(t *testing.T) {
	w := newWorld(t, map[string]bool{})
	token := linkFor(t, w, "form@example.com")
	for name, edit := range map[string]func(*Account){
		"no consent":     func(a *Account) { a.Accepted = false },
		"unknown locale": func(a *Account) { a.Locale = "it" },
		"no name":        func(a *Account) { a.Name = " " },
		"no password":    func(a *Account) { a.Password = "" },
	} {
		in := form(token)
		edit(&in)
		if _, err := w.svc.Finish(context.Background(), in); !errors.Is(err, ErrForm) {
			t.Errorf("%s = %v, want ErrForm", name, err)
		}
	}
}

// TestALinkBudgetIsPerAddress refuses a fourth link in an hour.
func TestALinkBudgetIsPerAddress(t *testing.T) {
	w := newWorld(t, map[string]bool{})
	for i := 0; i < MaxLinksPerHour; i++ {
		linkFor(t, w, "busy@example.com")
	}
	err := w.svc.Start(context.Background(), Request{Email: "busy@example.com", Proof: solve(t, w.svc)})
	if !errors.Is(err, ErrThrottled) {
		t.Fatalf("a fourth link = %v, want ErrThrottled", err)
	}
}
