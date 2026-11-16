package login

import (
	"net/http"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

func TestAnUnknownAuthRequestAnswersGoneEvenWithTheRightPassword(t *testing.T) {
	f := newFixture(t)
	user := f.user("gone@example.test", "correct horse battery staple")

	answer, response := f.login("no-such-flow", user.Email, "correct horse battery staple")
	if response.StatusCode != http.StatusGone || answer["error"] != "auth_request_gone" {
		t.Fatalf("an unknown auth request = %d %v, want 410 auth_request_gone", response.StatusCode, answer)
	}
	if cookie(response) != nil {
		t.Error("a gone request set an account cookie")
	}
}

func TestAnExpiredAuthRequestAnswersGoneEvenWithTheRightPassword(t *testing.T) {
	f := newFixture(t)
	user := f.user("expired@example.test", "correct horse battery staple")
	flow := f.authRequest()
	err := f.db.Model(&schemas.AuthRequest{}).Where("id = ?", flow).
		Update("expires_at", time.Now().UTC().Add(-time.Minute)).Error
	if err != nil {
		t.Fatalf("expiring the auth request: %v", err)
	}

	answer, response := f.login(flow, user.Email, "correct horse battery staple")
	if response.StatusCode != http.StatusGone || answer["error"] != "auth_request_gone" {
		t.Fatalf("an expired auth request = %d %v, want 410 auth_request_gone", response.StatusCode, answer)
	}
}

func TestAGoneAuthRequestAnswersTheSameForAnUnknownAddress(t *testing.T) {
	f := newFixture(t)

	answer, response := f.login("no-such-flow", "nobody@example.test", "wrong")
	if response.StatusCode != http.StatusGone || answer["error"] != "auth_request_gone" {
		t.Fatalf("a gone request with unknown credentials = %d %v, want 410 auth_request_gone", response.StatusCode, answer)
	}
}

func TestAWrongPasswordOnALiveRequestIsStillRefusedGenerically(t *testing.T) {
	f := newFixture(t)
	user := f.user("live@example.test", "correct horse battery staple")
	flow := f.authRequest()

	answer, response := f.login(flow, user.Email, "wrong")
	if response.StatusCode != http.StatusUnauthorized || answer["error"] != "invalid" {
		t.Fatalf("a wrong password = %d %v, want 401 invalid", response.StatusCode, answer)
	}
}

func TestResumingAGoneAuthRequestAnswersGone(t *testing.T) {
	f := newFixture(t)
	user := f.user("resume@example.test", "correct horse battery staple")
	_, first := f.login(f.authRequest(), user.Email, "correct horse battery staple")
	c := cookie(first)
	if c == nil {
		t.Fatal("expected account cookie from the first login")
	}

	answer, response := f.sessionGet(c, "no-such-flow")
	if response.StatusCode != http.StatusGone || answer["error"] != "auth_request_gone" {
		t.Fatalf("resuming a gone request = %d %v, want 410 auth_request_gone", response.StatusCode, answer)
	}
}
