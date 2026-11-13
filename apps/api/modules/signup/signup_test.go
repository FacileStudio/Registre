package signup

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/consent"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/flows"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/recovery"
	flow "github.com/FacileStudio/Registre/apps/api/internal/signup"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	altcha "github.com/altcha-org/altcha-lib-go"
	"github.com/go-chi/chi/v5"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func call(t *testing.T, router http.Handler, method, path, body string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.RemoteAddr = "192.0.2.1:1234"
	request.Header.Set("X-Facile-CSRF", "1")
	router.ServeHTTP(rec, request)
	return rec.Code, rec.Body.String()
}

// TestTheKillSwitchHidesEveryRoute: with sign-up off every route is a 404.
func TestTheKillSwitchHidesEveryRoute(t *testing.T) {
	router := chi.NewRouter()
	Mount(router, New(Deps{}, quiet()))
	for _, route := range [][2]string{
		{http.MethodGet, "/api/signup/challenge"}, {http.MethodPost, "/api/signup/start"},
		{http.MethodPost, "/api/signup/verify"}, {http.MethodPost, "/api/signup/finish"},
	} {
		if status, _ := call(t, router, route[0], route[1], `{}`); status != http.StatusNotFound {
			t.Errorf("%s %s with sign-up off = %d, want 404", route[0], route[1], status)
		}
	}
}

// open builds the routes over a real flow and the given budgets.
func open(t *testing.T, budget Limits) http.Handler {
	t.Helper()
	return openWith(t, budget, recovery.MailerFunc(func(context.Context, string, string, string) error { return nil }))
}

func openWith(t *testing.T, budget Limits, mailer recovery.Mailer) http.Handler {
	t.Helper()
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	db.Create(&schemas.User{ID: "known", Email: "known@example.com"})
	verifier := credentials.New(2)
	steps := flow.New(flow.Deps{
		DB: db, Verifier: verifier, Consents: consent.New(db, []byte("k")), Mailer: mailer,
	}, "an-altcha-hmac-key-of-at-least-32-bytes", "https://sso.example.com")
	router := chi.NewRouter()
	MountWithLimits(router, New(Deps{
		Flow: steps, Users: identity.New(db, verifier), Sessions: accounts.New(db), Cookies: accounts.NewCookies(false),
	}, quiet()), budget)
	return router
}

// proof fetches and solves a challenge over HTTP, as the page does.
func proof(t *testing.T, router http.Handler) string {
	t.Helper()
	_, body := call(t, router, http.MethodGet, "/api/signup/challenge", "")
	var challenge altcha.Challenge
	if err := json.Unmarshal([]byte(body), &challenge); err != nil {
		t.Fatalf("challenge: %v", err)
	}
	solution, _ := altcha.SolveChallenge(challenge.Challenge, challenge.Salt, altcha.SHA256, int(challenge.MaxNumber), 0, nil)
	payload, _ := json.Marshal(altcha.Payload{
		Algorithm: challenge.Algorithm, Challenge: challenge.Challenge, Number: int64(solution.Number),
		Salt: challenge.Salt, Signature: challenge.Signature,
	})
	return base64.StdEncoding.EncodeToString(payload)
}

// TestStartAnswersKnownAndUnknownIdentically compares the HTTP answers.
func TestStartAnswersKnownAndUnknownIdentically(t *testing.T) {
	router := open(t, DefaultLimits())
	var answers []string
	for _, email := range []string{"known@example.com", "unknown@example.com"} {
		status, body := call(t, router, http.MethodPost, "/api/signup/start",
			`{"email":"`+email+`","altcha":"`+proof(t, router)+`"}`)
		answers = append(answers, http.StatusText(status)+body)
	}
	if answers[0] != answers[1] || !strings.Contains(answers[0], "sent") {
		t.Fatalf("known %q, unknown %q: the answers must be identical", answers[0], answers[1])
	}
}

// TestGarbageProofsNeverSpendAStrangersBudget is the reviewer's lockout PoC:
// requests with a bad proof must not count against the address they name.
func TestGarbageProofsNeverSpendAStrangersBudget(t *testing.T) {
	budget := DefaultLimits()
	budget.StartBurst = 100
	router := open(t, budget)
	for i := 0; i < 10; i++ {
		if status, _ := call(t, router, http.MethodPost, "/api/signup/start", `{"email":"victim@example.com","altcha":"garbage"}`); status != http.StatusBadRequest {
			t.Fatalf("a garbage proof = %d, want 400", status)
		}
	}
	status, _ := call(t, router, http.MethodPost, "/api/signup/start",
		`{"email":"victim@example.com","altcha":"`+proof(t, router)+`"}`)
	if status != http.StatusAccepted {
		t.Fatalf("the victim with a valid proof after garbage starts = %d, want 202", status)
	}
}

// TestStartIsLimitedPerEmailAfterTheProof caps the links one address gets.
func TestStartIsLimitedPerEmailAfterTheProof(t *testing.T) {
	budget := DefaultLimits()
	budget.StartBurst = 100
	router := open(t, budget)
	for i := 0; i < flow.MaxLinksPerHour; i++ {
		status, _ := call(t, router, http.MethodPost, "/api/signup/start", `{"email":"a@example.com","altcha":"`+proof(t, router)+`"}`)
		if status != http.StatusAccepted {
			t.Fatalf("start %d = %d, want 202", i+1, status)
		}
	}
	status, body := call(t, router, http.MethodPost, "/api/signup/start", `{"email":"A@example.com","altcha":"`+proof(t, router)+`"}`)
	if status != http.StatusTooManyRequests || !strings.Contains(body, `"retry_after":`) {
		t.Fatalf("one link too many = %d %s, want 429 with retry_after", status, body)
	}
}

// TestStartIsLimitedPerAddress.
func TestStartIsLimitedPerAddress(t *testing.T) {
	budget := DefaultLimits()
	budget.StartLimit, budget.StartBurst = 0.0001, 2
	router := open(t, budget)
	for _, email := range []string{"c@example.com", "d@example.com"} {
		call(t, router, http.MethodPost, "/api/signup/start", `{"email":"`+email+`","altcha":"x"}`)
	}
	if status, _ := call(t, router, http.MethodPost, "/api/signup/start", `{"email":"e@example.com","altcha":"x"}`); status != http.StatusTooManyRequests {
		t.Fatalf("third start from one address = %d, want 429", status)
	}
}

// carry is one sign-up started and finished by browsers holding the given
// flow cookies, for an app login bound to the flow cookie "bound-secret".
type carry struct {
	startCookie, finishCookie string
	named                     string
}

// signUpCarrying runs start and finish with those cookies and answers the
// finish body and the auth request's state afterwards.
func signUpCarrying(t *testing.T, c carry) (string, schemas.AuthRequest) {
	t.Helper()
	router := open(t, DefaultLimits())
	url, _ := testdb.URL()
	db, _ := testdb.Open(url)
	db.Create(&schemas.AuthRequest{ID: "R", ClientID: "app", RedirectURI: "https://app/cb",
		ExpiresAt: time.Now().Add(10 * time.Minute), BrowserHash: flows.Digest("bound-secret")})
	start := `{"email":"new@example.com","altcha":"` + proof(t, router) + `","auth_request":"` + c.named + `"}`
	if status, _ := callWithFlow(t, router, "/api/signup/start", start, c.startCookie); status != http.StatusAccepted {
		t.Fatalf("start = %d", status)
	}
	sum := sha256.Sum256([]byte("new-token"))
	db.Model(&schemas.SignupChallenge{}).Where("email = ?", "new@example.com").Update("token_hash", hex.EncodeToString(sum[:]))
	_, body := callWithFlow(t, router, "/api/signup/finish",
		`{"token":"new-token","name":"New","locale":"en","accept_terms":true,"password":"correct horse battery staple zz"}`, c.finishCookie)
	var pending schemas.AuthRequest
	db.Where("id = ?", "R").Take(&pending)
	return body, pending
}

// callWithFlow posts body with a flow cookie holding secret, when one is given.
func callWithFlow(t *testing.T, router http.Handler, path, body, secret string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.RemoteAddr = "192.0.2.1:1234"
	request.Header.Set("X-Facile-CSRF", "1")
	if secret != "" {
		request.AddCookie(&http.Cookie{Name: flows.CookieName(flows.Digest(secret), false), Value: secret})
	}
	router.ServeHTTP(rec, request)
	return rec.Code, rec.Body.String()
}

// TestABoundLoginIsCarriedThroughSignup finishes the app login when the same
// browser that started it starts and finishes sign-up.
func TestABoundLoginIsCarriedThroughSignup(t *testing.T) {
	body, pending := signUpCarrying(t, carry{startCookie: "bound-secret", finishCookie: "bound-secret", named: "R"})
	if !strings.Contains(body, "/authorize/callback?id=R") || pending.UserID == nil {
		t.Fatalf("finish = %s, request %+v; want the bound login completed", body, pending)
	}
}

// TestAForeignLoginIsNeverCarried is the reviewer's PoC: naming an auth
// request another browser started gets nothing completed.
func TestAForeignLoginIsNeverCarried(t *testing.T) {
	body, pending := signUpCarrying(t, carry{startCookie: "attacker-secret", finishCookie: "bound-secret", named: "R"})
	if strings.Contains(body, "attacker") || strings.Contains(body, "callback") || pending.UserID != nil {
		t.Fatalf("finish = %s, request %+v; want Registre's home and nothing completed", body, pending)
	}
}

// TestALinkOpenedElsewhereDoesNotFinishTheLogin refuses a finishing browser
// that does not hold the flow cookie, even when the start was bound.
func TestALinkOpenedElsewhereDoesNotFinishTheLogin(t *testing.T) {
	for name, finish := range map[string]string{"unbound": "", "foreign": "attacker-secret"} {
		t.Run(name, func(t *testing.T) {
			body, pending := signUpCarrying(t, carry{startCookie: "bound-secret", finishCookie: finish, named: "R"})
			if strings.Contains(body, "callback") || pending.UserID != nil {
				t.Fatalf("finish = %s, request %+v; want Registre's home and nothing completed", body, pending)
			}
		})
	}
}

// TestFinishWantsTheCSRFHeader refuses a sign-up a cross-site page submits.
func TestFinishWantsTheCSRFHeader(t *testing.T) {
	router := open(t, DefaultLimits())
	rec := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/signup/finish", strings.NewReader(`{}`))
	router.ServeHTTP(rec, request)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("finish without the header = %d, want 403", rec.Code)
	}
}

// TestSignupIsPasswordOnly: the passkey ceremony route is gone, a finish
// carrying a passkey answer and no password is refused, seven characters
// are refused with the minimum named, and eight are accepted.
func TestSignupIsPasswordOnly(t *testing.T) {
	router := open(t, DefaultLimits())
	if status, _ := callWithFlow(t, router, "/api/signup/passkey", `{"token":"t"}`, ""); status != http.StatusNotFound {
		t.Fatalf("POST /api/signup/passkey = %d, want 404", status)
	}
	url, _ := testdb.URL()
	db, _ := testdb.Open(url)
	start := `{"email":"short@example.com","altcha":"` + proof(t, router) + `"}`
	if status, _ := callWithFlow(t, router, "/api/signup/start", start, ""); status != http.StatusAccepted {
		t.Fatalf("start = %d", status)
	}
	sum := sha256.Sum256([]byte("short-token"))
	db.Model(&schemas.SignupChallenge{}).Where("email = ?", "short@example.com").Update("token_hash", hex.EncodeToString(sum[:]))
	form := `{"token":"short-token","name":"Short","locale":"en","accept_terms":true`
	cases := []struct {
		extra, want string
		status      int
	}{
		{`,"ceremony":"c","credential":{}}`, `"incomplete"`, http.StatusBadRequest},
		{`,"password":"seven c"}`, `"minimum":8`, http.StatusBadRequest},
		{`,"password":"eightchr"}`, `"redirect"`, http.StatusCreated},
	}
	for _, c := range cases {
		status, body := callWithFlow(t, router, "/api/signup/finish", form+c.extra, "")
		if status != c.status || !strings.Contains(body, c.want) {
			t.Fatalf("finish with %s = %d %s, want %d with %s", c.extra, status, body, c.status, c.want)
		}
	}
}
