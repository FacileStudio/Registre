package login

import (
	"net/http"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/challenges"
	"github.com/FacileStudio/Registre/apps/api/schemas"
)

func TestAnAccountWithoutAFactorStillGetsItsRedirect(t *testing.T) {
	f := newFixture(t)
	user := f.user("plain@example.test", "correct horse battery staple")
	flow := f.authRequest()

	answer, response := f.login(flow, user.Email, "correct horse battery staple")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/login = %d, want 200", response.StatusCode)
	}
	if answer["redirect"] != "/authorize/callback?id="+flow {
		t.Fatalf("redirect = %q, want the callback for %s", answer["redirect"], flow)
	}
	if answer["challenge"] != "" {
		t.Error("an account with no second factor was handed a challenge")
	}
	if cookie(response) == nil {
		t.Error("no account cookie: the account pages would be unreachable straight after logging in")
	}
	if !f.done(flow) {
		t.Error("the flow is not authenticated, so the callback will refuse it")
	}
}

func TestAnEnrolledAccountGetsAChallengeAndNothingElse(t *testing.T) {
	f := newFixture(t)
	user := f.user("enrolled@example.test", "correct horse battery staple")
	f.enrol(user)
	flow := f.authRequest()

	answer, response := f.login(flow, user.Email, "correct horse battery staple")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/login = %d, want 200", response.StatusCode)
	}
	if answer["challenge"] == "" {
		t.Fatal("no challenge: the second factor has nothing to be answered against")
	}
	if answer["redirect"] != "" {
		t.Fatal("a password alone earned a redirect from an account that has a second factor")
	}
	if cookie(response) != nil {
		t.Error("an account cookie was set before the second factor was proven")
	}
	if f.done(flow) {
		t.Error("the flow was authenticated on the password alone")
	}
}

func TestAWrongPasswordAnswersTheSameWhoeverHasAFactor(t *testing.T) {
	f := newFixture(t)
	user := f.user("enrolled@example.test", "correct horse battery staple")
	f.enrol(user)
	flow := f.authRequest()

	known, response := f.login(flow, user.Email, "wrong")
	if response.StatusCode != http.StatusUnauthorized || known["error"] != "invalid" {
		t.Fatalf("a wrong password = %d %v, want 401 and the generic refusal", response.StatusCode, known)
	}
	unknown, response := f.login(flow, "nobody@example.test", "wrong")
	if response.StatusCode != http.StatusUnauthorized || unknown["error"] != "invalid" {
		t.Fatalf("an unknown address = %d %v, want the same refusal", response.StatusCode, unknown)
	}
}

func TestAGoodCodeCompletesTheLogin(t *testing.T) {
	f := newFixture(t)
	user := f.user("enrolled@example.test", "correct horse battery staple")
	secret, _ := f.enrol(user)
	flow := f.authRequest()

	first, _ := f.login(flow, user.Email, "correct horse battery staple")
	answer, response := f.second(first["challenge"], f.code(secret))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/login/totp = %d, want 200: %v", response.StatusCode, answer)
	}
	if answer["redirect"] != "/authorize/callback?id="+flow {
		t.Fatalf("redirect = %q, want the callback for %s", answer["redirect"], flow)
	}
	if cookie(response) == nil {
		t.Error("no account cookie on the completed login")
	}
	if !f.done(flow) {
		t.Error("the flow is still pending after a proven second factor")
	}
	if _, again := f.second(first["challenge"], f.code(secret)); again.StatusCode != http.StatusUnauthorized {
		t.Errorf("the spent challenge answered %d, want 401", again.StatusCode)
	}
}

func TestARecoveryCodeCompletesTheLogin(t *testing.T) {
	f := newFixture(t)
	user := f.user("enrolled@example.test", "correct horse battery staple")
	_, recovery := f.enrol(user)
	flow := f.authRequest()

	first, _ := f.login(flow, user.Email, "correct horse battery staple")
	answer, response := f.second(first["challenge"], recovery[0])
	if response.StatusCode != http.StatusOK {
		t.Fatalf("a recovery code = %d, want 200: %v", response.StatusCode, answer)
	}
	if !f.done(flow) {
		t.Fatal("the flow is still pending after a spent recovery code")
	}
	var spent int64
	err := f.db.Model(&schemas.RecoveryCode{}).Where("used_at IS NOT NULL").Count(&spent).Error
	if err != nil {
		t.Fatalf("counting the spent codes: %v", err)
	}
	if spent != 1 {
		t.Errorf("%d recovery codes spent, want 1", spent)
	}
}

func TestWrongCodesBurnTheAttemptBudget(t *testing.T) {
	f := newFixture(t)
	user := f.user("enrolled@example.test", "correct horse battery staple")
	secret, _ := f.enrol(user)
	flow := f.authRequest()

	first, _ := f.login(flow, user.Email, "correct horse battery staple")
	answer, response := f.second(first["challenge"], "000000")
	if response.StatusCode != http.StatusUnauthorized || answer["error"] != "invalid" {
		t.Fatalf("a wrong code = %d %v, want 401 and the generic refusal", response.StatusCode, answer)
	}
	var row schemas.LoginChallenge
	if err := f.db.Take(&row).Error; err != nil {
		t.Fatalf("reading the challenge: %v", err)
	}
	if row.Attempts != 1 {
		t.Fatalf("attempts = %d after one wrong code, want 1", row.Attempts)
	}
	for i := row.Attempts; i < challenges.MaxAttempts; i++ {
		f.second(first["challenge"], "000000")
	}
	if _, locked := f.second(first["challenge"], f.code(secret)); locked.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a good code past the budget = %d, want 401", locked.StatusCode)
	}
	if f.done(flow) {
		t.Fatal("the flow completed after the challenge ran out of attempts")
	}
}

func TestAChallengeCannotCompleteAnotherFlow(t *testing.T) {
	f := newFixture(t)
	user := f.user("enrolled@example.test", "correct horse battery staple")
	secret, _ := f.enrol(user)
	mine, theirs := f.authRequest(), f.authRequest()

	first, _ := f.login(mine, user.Email, "correct horse battery staple")
	if _, response := f.second(first["challenge"], f.code(secret)); response.StatusCode != http.StatusOK {
		t.Fatalf("answering the challenge = %d, want 200", response.StatusCode)
	}
	if !f.done(mine) {
		t.Fatal("the flow the challenge was minted for is still pending")
	}
	if f.done(theirs) {
		t.Fatal("a challenge minted for one flow authenticated another")
	}
}

func TestDirectLoginWithoutAuthRequest(t *testing.T) {
	f := newFixture(t)
	user := f.user("direct@example.test", "correct horse battery staple")

	answer, response := f.login("", user.Email, "correct horse battery staple")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/login = %d, want 200", response.StatusCode)
	}
	if answer["redirect"] != "/" {
		t.Fatalf("redirect = %q, want /", answer["redirect"])
	}
	if answer["challenge"] != "" {
		t.Error("an account with no second factor was handed a challenge")
	}
	if cookie(response) == nil {
		t.Error("no account cookie set on direct login")
	}
}

func TestDirectLoginWithTwoFactor(t *testing.T) {
	f := newFixture(t)
	user := f.user("direct2fa@example.test", "correct horse battery staple")
	secret, _ := f.enrol(user)

	first, response := f.login("", user.Email, "correct horse battery staple")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/login = %d, want 200", response.StatusCode)
	}
	if first["challenge"] == "" {
		t.Fatal("no challenge returned for 2fa direct login")
	}
	answer, secondResponse := f.second(first["challenge"], f.code(secret))
	if secondResponse.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/login/totp = %d, want 200", secondResponse.StatusCode)
	}
	if answer["redirect"] != "/" {
		t.Fatalf("redirect = %q, want /", answer["redirect"])
	}
	if cookie(secondResponse) == nil {
		t.Error("no account cookie set after completing direct 2fa login")
	}
}
