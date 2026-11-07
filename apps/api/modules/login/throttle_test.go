package login

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/throttle"
	"gorm.io/gorm"
)

const goodPassword = "correct horse battery staple"

func throttled(t *testing.T) *fixture {
	t.Helper()
	return newFixtureWith(t, func(s *Service, db *gorm.DB) { s.WithThrottle(throttle.New(db)) })
}

func TestGuessingOneAccountIsThrottledAfterFiveFailuresWithoutLockingIt(t *testing.T) {
	f := throttled(t)
	user := f.user("victim@example.test", goodPassword)
	flow := f.authRequest()

	for i := 1; i <= 5; i++ {
		if _, response := f.login(flow, user.Email, "wrong password "+strconv.Itoa(i)); response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("free attempt %d = %d, want 401", i, response.StatusCode)
		}
	}
	answer, response := f.login(flow, user.Email, "wrong password 6")
	if response.StatusCode != http.StatusTooManyRequests || answer["error"] != "too_many_attempts" {
		t.Fatalf("attempt 6 = %d %v, want 429 too_many_attempts", response.StatusCode, answer)
	}
	if response.Header.Get("Retry-After") != "30" {
		t.Errorf("Retry-After = %q, want 30", response.Header.Get("Retry-After"))
	}
	if _, response := f.login(flow, user.Email, goodPassword); response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("the right password during the delay = %d, want 429: the delay must bind the guesser's lucky guess too", response.StatusCode)
	}
}

func TestAnUnknownAddressIsThrottledByteForByteLikeAKnownOne(t *testing.T) {
	f := throttled(t)
	f.user("known@example.test", goodPassword)
	flow := f.authRequest()

	run := func(email string) (int, map[string]string, string) {
		for range 5 {
			f.login(flow, email, "wrong password")
		}
		answer, response := f.login(flow, email, "wrong password")
		return response.StatusCode, answer, response.Header.Get("Retry-After")
	}
	knownStatus, knownBody, knownRetry := run("known@example.test")
	unknownStatus, unknownBody, unknownRetry := run("nobody@example.test")
	if knownStatus != unknownStatus || knownBody["error"] != unknownBody["error"] || knownRetry != unknownRetry {
		t.Fatalf("known %d %v %q, unknown %d %v %q: the throttle is an account oracle",
			knownStatus, knownBody, knownRetry, unknownStatus, unknownBody, unknownRetry)
	}
	if knownStatus != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", knownStatus)
	}
}

func TestTheThrottleKeysOnTheNormalisedAddress(t *testing.T) {
	f := throttled(t)
	f.user("case@example.test", goodPassword)
	flow := f.authRequest()
	variants := []string{"case@example.test", "CASE@example.test", " Case@Example.test ", "case@EXAMPLE.test", "CASE@EXAMPLE.TEST"}
	for _, email := range variants {
		f.login(flow, email, "wrong password")
	}
	if _, response := f.login(flow, "case@example.test", "wrong password"); response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("spelling variants dodged the throttle: %d", response.StatusCode)
	}
}

func TestASuccessResetsTheCounter(t *testing.T) {
	f := throttled(t)
	user := f.user("reset@example.test", goodPassword)
	flow := f.authRequest()
	for range 4 {
		f.login(flow, user.Email, "wrong password")
	}
	if _, response := f.login(flow, user.Email, goodPassword); response.StatusCode != http.StatusOK {
		t.Fatalf("the right password on attempt 5 = %d, want 200", response.StatusCode)
	}
	for i := 1; i <= 5; i++ {
		if _, response := f.login(flow, user.Email, "wrong password"); response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d after a success = %d, want a fresh budget of 401s", i, response.StatusCode)
		}
	}
}

func TestOneAccountsFailuresDoNotThrottleAnother(t *testing.T) {
	f := throttled(t)
	f.user("a@example.test", goodPassword)
	other := f.user("b@example.test", goodPassword)
	flow := f.authRequest()
	for range 8 {
		f.login(flow, "a@example.test", "wrong password")
	}
	if _, response := f.login(flow, other.Email, goodPassword); response.StatusCode != http.StatusOK {
		t.Fatalf("a bystander's login = %d, want 200", response.StatusCode)
	}
}

func TestTheSecondFactorStepIsThrottledPerAccount(t *testing.T) {
	f := throttled(t)
	user := f.user("totp@example.test", goodPassword)
	secret, _ := f.enrol(user)
	flow := f.authRequest()

	attempts := 0
	for ; attempts < 20; attempts++ {
		answer, response := f.login(flow, user.Email, goodPassword)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("password step = %d: the second-factor throttle must not depend on the password key", response.StatusCode)
		}
		_, second := f.second(answer["challenge"], "000000")
		if second.StatusCode == http.StatusTooManyRequests {
			break
		}
		if second.StatusCode != http.StatusUnauthorized {
			t.Fatalf("wrong code = %d, want 401", second.StatusCode)
		}
	}
	if attempts != 5 {
		t.Fatalf("throttled after %d wrong codes, want 5 free then a 429 on the 6th", attempts)
	}
	answer, _ := f.login(flow, user.Email, goodPassword)
	if _, response := f.second(answer["challenge"], f.code(secret)); response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("the right code during the delay = %d, want 429 despite fresh password logins", response.StatusCode)
	}
}

func TestASaturatedVerifierAnswers503WithRetryAfter(t *testing.T) {
	recorder := newRecorder()
	if !(&Service{}).refuseOverload(recorder, errBusyForTest()) {
		t.Fatal("ErrBusy was not handled as overload")
	}
	if recorder.Code != http.StatusServiceUnavailable || recorder.Header().Get("Retry-After") == "" {
		t.Fatalf("answer = %d Retry-After %q, want 503 with a Retry-After", recorder.Code, recorder.Header().Get("Retry-After"))
	}
}
