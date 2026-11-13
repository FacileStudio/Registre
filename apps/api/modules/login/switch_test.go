package login

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

// logouts records the back-channel logouts the account service fires.
type logouts chan string

// Fire records whose session was announced as ended.
func (l logouts) Fire(_ context.Context, userID string, _ []string) error {
	l <- userID
	return nil
}

// switchFixture is the login fixture whose account service announces ended
// sessions to the returned channel.
func switchFixture(t *testing.T) (*fixture, logouts) {
	t.Helper()
	fired := make(logouts, 8)
	f := newFixtureWith(t, func(s *Service, db *gorm.DB) {
		s.factor.Accounts = accounts.New(db).WithNotifier(fired, nil)
	})
	return f, fired
}

// signIn logs email in to a fresh flow, carrying held when the browser
// already has an account cookie, and returns the new cookie.
func (f *fixture) signIn(email string, held *http.Cookie) *http.Cookie {
	f.t.Helper()
	body := map[string]string{"auth_request": f.authRequest(), "email": email, "password": "correct horse battery staple"}
	var answer map[string]string
	var response *http.Response
	if held == nil {
		answer, response = f.post("/api/login", body)
	} else {
		answer, response = f.postWith("/api/login", body, held)
	}
	if response.StatusCode != http.StatusOK {
		f.t.Fatalf("signing in %s answered %d: %v", email, response.StatusCode, answer)
	}
	return cookie(response)
}

// liveSessions counts the sessions a user still holds.
func (f *fixture) liveSessions(userID string) int64 {
	f.t.Helper()
	var count int64
	err := f.db.Model(&schemas.Session{}).Where("user_id = ? AND ended_at IS NULL", userID).Count(&count).Error
	if err != nil {
		f.t.Fatalf("counting sessions: %v", err)
	}
	return count
}

func TestSigningInAsAnotherPersonSignsThePreviousOneOutOfTheApps(t *testing.T) {
	f, fired := switchFixture(t)
	alice := f.user("alice@example.test", "correct horse battery staple")
	bob := f.user("bob@example.test", "correct horse battery staple")

	held := f.signIn(alice.Email, nil)
	f.signIn(bob.Email, held)

	select {
	case userID := <-fired:
		if userID != alice.ID {
			t.Fatalf("fired for %s, want alice", userID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("alice's apps were never told her session ended")
	}
	select {
	case userID := <-fired:
		t.Fatalf("a second logout was fired, for %s", userID)
	case <-time.After(200 * time.Millisecond):
	}
	if f.liveSessions(alice.ID) != 0 || f.liveSessions(bob.ID) != 1 {
		t.Fatalf("alice holds %d live sessions and bob %d, want 0 and 1",
			f.liveSessions(alice.ID), f.liveSessions(bob.ID))
	}
}

func TestSigningInAgainAsTheSamePersonSignsNothingOut(t *testing.T) {
	f, fired := switchFixture(t)
	alice := f.user("alice@example.test", "correct horse battery staple")

	held := f.signIn(alice.Email, nil)
	f.signIn(alice.Email, held)

	select {
	case userID := <-fired:
		t.Fatalf("signing in again fired a logout for %s", userID)
	case <-time.After(200 * time.Millisecond):
	}
	if got := f.liveSessions(alice.ID); got != 2 {
		t.Fatalf("alice holds %d live sessions, want both", got)
	}
}
