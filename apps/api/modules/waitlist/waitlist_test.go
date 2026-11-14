package waitlist

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	flow "github.com/FacileStudio/Registre/apps/api/internal/signup"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"
	"github.com/FacileStudio/Registre/apps/api/internal/waitlist"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	altcha "github.com/altcha-org/altcha-lib-go"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

const testKey = "an-altcha-hmac-key-of-at-least-32-bytes"

const scribe = "https://scribe.facile.studio"

type world struct {
	router http.Handler
	db     *gorm.DB
	list   *waitlist.Service
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func build(t *testing.T, budget Limits) world {
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
	list := waitlist.New(db, []byte(testKey))
	proofs := flow.New(flow.Deps{DB: db}, testKey, "https://sso.example.com")
	router := chi.NewRouter()
	router.Use(CORS(DefaultOrigins(), func(next http.Handler) http.Handler { return next }))
	MountWithLimits(router, New(list, proofs, DefaultOrigins(), quiet()), budget)
	return world{router: router, db: db, list: list}
}

func roomy() Limits { return Limits{JoinLimit: 100, JoinBurst: 100, StepLimit: 100, StepBurst: 100} }

func send(w world, method, path, origin, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.RemoteAddr = "192.0.2.1:1234"
	request.Header.Set("Content-Type", "application/json")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	w.router.ServeHTTP(rec, request)
	return rec
}

func proof(t *testing.T, w world) string {
	t.Helper()
	rec := send(w, http.MethodGet, "/api/waitlist/challenge", scribe, "")
	var challenge altcha.Challenge
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &challenge) != nil {
		t.Fatalf("challenge: %d %s", rec.Code, rec.Body)
	}
	solution, err := altcha.SolveChallenge(challenge.Challenge, challenge.Salt, altcha.SHA256, int(challenge.MaxNumber), 0, nil)
	if err != nil || solution == nil {
		t.Fatalf("solving: %v", err)
	}
	payload, _ := json.Marshal(altcha.Payload{
		Algorithm: challenge.Algorithm, Challenge: challenge.Challenge, Number: int64(solution.Number),
		Salt: challenge.Salt, Signature: challenge.Signature,
	})
	return base64.StdEncoding.EncodeToString(payload)
}

func joinBodyFor(email, product, altchaProof string) string {
	return fmt.Sprintf(`{"email":%q,"product":%q,"locale":"fr","consent":"2026-11-14","altcha":%q}`,
		email, product, altchaProof)
}

func rows(t *testing.T, w world) []schemas.WaitlistEntry {
	t.Helper()
	all, err := w.list.List(t.Context(), "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return all
}

func TestOffHidesEveryRoute(t *testing.T) {
	router := chi.NewRouter()
	Mount(router, New(nil, nil, DefaultOrigins(), quiet()))
	w := world{router: router}
	for _, route := range [][2]string{{http.MethodGet, "/api/waitlist/challenge"}, {http.MethodPost, "/api/waitlist"}, {http.MethodPost, "/api/waitlist/unsubscribe"}} {
		if rec := send(w, route[0], route[1], "", `{}`); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s = %d, want 404", route[0], route[1], rec.Code)
		}
	}
}

func TestPreflightAllowsOnlyTheLandings(t *testing.T) {
	w := build(t, roomy())
	for _, origin := range DefaultOrigins() {
		rec := send(w, http.MethodOptions, "/api/waitlist", origin, "")
		if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != origin {
			t.Errorf("%s: %d, allow-origin %q", origin, rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
		}
		if rec.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Errorf("%s: credentials allowed", origin)
		}
	}
	rec := send(w, http.MethodOptions, "/api/waitlist", "https://evil.example", "")
	if rec.Code != http.StatusForbidden || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("evil preflight: %d, allow-origin %q", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestAnotherOriginCannotWrite(t *testing.T) {
	w := build(t, roomy())
	rec := send(w, http.MethodPost, "/api/waitlist", "https://evil.example", joinBodyFor("p@example.com", "scribe", proof(t, w)))
	if rec.Code != http.StatusForbidden || len(rows(t, w)) != 0 {
		t.Fatalf("evil post: %d, %d rows", rec.Code, len(rows(t, w)))
	}
}

func TestJoinStoresOneRowAndAnswersDuplicatesAlike(t *testing.T) {
	w := build(t, roomy())
	for _, email := range []string{"person@example.com", "Person@Example.com"} {
		rec := send(w, http.MethodPost, "/api/waitlist", scribe, joinBodyFor(email, "scribe", proof(t, w)))
		if rec.Code != http.StatusAccepted || rec.Header().Get("Access-Control-Allow-Origin") != scribe {
			t.Fatalf("join %s: %d %s", email, rec.Code, rec.Body)
		}
	}
	got := rows(t, w)
	if len(got) != 1 || got[0].Email != "person@example.com" || got[0].Locale != "fr" || got[0].ConsentVersion != "2026-11-14" {
		t.Fatalf("rows: %+v", got)
	}
}

func TestJoinRefusesBadInput(t *testing.T) {
	w := build(t, roomy())
	cases := map[string]string{
		"invalid_email":    joinBodyFor("not-an-address", "scribe", proof(t, w)),
		"invalid_product":  joinBodyFor("p@example.com", "field", proof(t, w)),
		"invalid_proof":    joinBodyFor("p@example.com", "scribe", "bm90LWEtcHJvb2Y="),
		"consent_required": `{"email":"p@example.com","product":"scribe","locale":"fr","altcha":"x"}`,
		"invalid":          `{"email":"p@example.com","name":"Someone"}`,
	}
	for want, body := range cases {
		rec := send(w, http.MethodPost, "/api/waitlist", scribe, body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"`+want+`"`) {
			t.Errorf("%s: %d %s", want, rec.Code, rec.Body)
		}
	}
	if len(rows(t, w)) != 0 {
		t.Fatal("a refused request wrote a row")
	}
}

func TestAProofIsSpentOnce(t *testing.T) {
	w := build(t, roomy())
	spent := proof(t, w)
	if rec := send(w, http.MethodPost, "/api/waitlist", scribe, joinBodyFor("a@example.com", "scribe", spent)); rec.Code != http.StatusAccepted {
		t.Fatalf("first: %d", rec.Code)
	}
	if rec := send(w, http.MethodPost, "/api/waitlist", scribe, joinBodyFor("b@example.com", "scribe", spent)); rec.Code != http.StatusBadRequest {
		t.Fatalf("replay: %d", rec.Code)
	}
}

func TestJoinIsRateLimitedPerAddress(t *testing.T) {
	w := build(t, Limits{JoinLimit: 0.001, JoinBurst: 2, StepLimit: 100, StepBurst: 100})
	codes := make([]int, 0, 3)
	for range 3 {
		codes = append(codes, send(w, http.MethodPost, "/api/waitlist", scribe, `{}`).Code)
	}
	if codes[2] != http.StatusTooManyRequests {
		t.Fatalf("codes %v, want the third to be 429", codes)
	}
}

func TestUnsubscribeByQueryOrBody(t *testing.T) {
	w := build(t, roomy())
	for _, email := range []string{"a@example.com", "b@example.com"} {
		send(w, http.MethodPost, "/api/waitlist", scribe, joinBodyFor(email, "pastille", proof(t, w)))
	}
	got := rows(t, w)
	if len(got) != 2 {
		t.Fatalf("%d rows, want 2", len(got))
	}
	if rec := send(w, http.MethodPost, "/api/waitlist/unsubscribe?token="+w.list.Token(got[0].ID), "", "List-Unsubscribe=One-Click"); rec.Code != http.StatusOK {
		t.Fatalf("one-click: %d %s", rec.Code, rec.Body)
	}
	if rec := send(w, http.MethodPost, "/api/waitlist/unsubscribe", "", `{"token":"`+w.list.Token(got[1].ID)+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("json: %d %s", rec.Code, rec.Body)
	}
	if rec := send(w, http.MethodPost, "/api/waitlist/unsubscribe", "", `{"token":"forged.token"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("forged: %d", rec.Code)
	}
	if len(rows(t, w)) != 0 {
		t.Fatal("rows left after unsubscribing")
	}
}

func TestOtherPathsKeepTheFallbackPolicy(t *testing.T) {
	called := false
	handler := CORS(DefaultOrigins(), func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; next.ServeHTTP(w, r) })
	})(http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodOptions, "/api/waitlistx", nil)
	request.Header.Set("Origin", scribe)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if !called {
		t.Fatal("a path outside the waitlist skipped the fallback policy")
	}
}
