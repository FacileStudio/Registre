package login

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
	"github.com/FacileStudio/Registre/apps/api/internal/social"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	"gorm.io/gorm"
)

type fakeProvider struct{ profile social.Profile }

func (fakeProvider) Name() string { return "discord" }
func (fakeProvider) AuthURL(state, _, _ string) string {
	return "https://discord.test/authorize?state=" + url.QueryEscape(state)
}
func (f fakeProvider) Exchange(context.Context, string, string, string) (social.Profile, error) {
	return f.profile, nil
}

func socialFixture(t *testing.T, profile social.Profile, signupOpen bool) *fixture {
	t.Helper()
	return newFixtureWith(t, func(service *Service, db *gorm.DB) {
		service.WithSocial(social.New(db, []social.Provider{fakeProvider{profile}},
			social.Config{Issuer: "https://sso.test", SignupOpen: signupOpen}))
		row := schemas.Client{
			ClientID: "app", Subject: "app-subject", Name: "App", Kind: schemas.KindApplication,
			LoginProviders: schemas.StringList{"discord"},
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("creating the client: %v", err)
		}
	})
}

func (f *fixture) get(path string, cookies ...*http.Cookie) *http.Response {
	f.t.Helper()
	request := httptest.NewRequest(http.MethodGet, f.server.URL+path, nil)
	request.RequestURI = ""
	for _, c := range cookies {
		request.AddCookie(c)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		f.t.Fatalf("GET %s: %v", path, err)
	}
	f.t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func stateOf(t *testing.T, response *http.Response) (string, *http.Cookie) {
	t.Helper()
	if response.StatusCode != http.StatusFound {
		t.Fatalf("start answered %d, want 302", response.StatusCode)
	}
	location, err := url.Parse(response.Header.Get("Location"))
	if err != nil || location.Host != "discord.test" {
		t.Fatalf("start redirected to %q", response.Header.Get("Location"))
	}
	for _, c := range response.Cookies() {
		if c.Name == "registre-social" && c.Value != "" {
			return location.Query().Get("state"), c
		}
	}
	t.Fatal("start set no state cookie")
	return "", nil
}

func TestProvidersListsOnlyWhatTheClientOptedInto(t *testing.T) {
	f := socialFixture(t, social.Profile{}, false)
	id := f.authRequest()
	response := f.get("/api/login/providers?auth_request=" + id)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status %d", response.StatusCode)
	}
	if got := readBody(t, response); !strings.Contains(got, `"discord"`) {
		t.Errorf("providers = %s, want discord", got)
	}
	if got := readBody(t, f.get("/api/login/providers")); strings.Contains(got, "discord") {
		t.Errorf("a login with no auth request offered %s", got)
	}
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("reading the body: %v", err)
	}
	return string(raw)
}

func TestADiscordSignInCreatesTheAccountAndFinishesTheFlow(t *testing.T) {
	f := socialFixture(t, social.Profile{Subject: "42", Email: "new@facile.studio", EmailVerified: true, Name: "New"}, true)
	id := f.authRequest()
	state, held := stateOf(t, f.get("/api/login/social/discord/start?auth_request="+id))

	response := f.get("/api/login/social/discord/callback?code=c&state="+url.QueryEscape(state), held)
	if response.StatusCode != http.StatusFound || !strings.HasPrefix(response.Header.Get("Location"), "/authorize/callback?id="+id) {
		t.Fatalf("callback = %d %q, want a redirect to the app callback", response.StatusCode, response.Header.Get("Location"))
	}
	if !f.done(id) || cookie(response) == nil {
		t.Fatal("the flow is not finished or no account cookie was set")
	}
}

func TestACallbackWithoutTheBrowsersStateCookieFinishesNothing(t *testing.T) {
	f := socialFixture(t, social.Profile{Subject: "43", Email: "x@facile.studio", EmailVerified: true}, true)
	id := f.authRequest()
	state, _ := stateOf(t, f.get("/api/login/social/discord/start?auth_request="+id))

	response := f.get("/api/login/social/discord/callback?code=c&state=" + url.QueryEscape(state))
	if !strings.HasPrefix(response.Header.Get("Location"), "/login?error=") {
		t.Fatalf("Location = %q, want the login page with an error", response.Header.Get("Location"))
	}
	if f.done(id) || cookie(response) != nil {
		t.Fatal("a callback from another browser finished the flow")
	}
}

func TestAnEnrolledAccountStillOwesItsSecondFactorAfterDiscord(t *testing.T) {
	f := socialFixture(t, social.Profile{Subject: "44", Email: "yann@facile.studio", EmailVerified: true}, false)
	user := f.user("yann@facile.studio", "correct horse battery staple")
	secret, _ := f.enrol(user)
	id := f.authRequest()
	state, held := stateOf(t, f.get("/api/login/social/discord/start?auth_request="+id))

	response := f.get("/api/login/social/discord/callback?code=c&state="+url.QueryEscape(state), held)
	location, err := url.Parse(response.Header.Get("Location"))
	if err != nil || location.Path != "/login" || location.Query().Get("second_factor") != "1" {
		t.Fatalf("Location = %q, want /login at the second factor", response.Header.Get("Location"))
	}
	if strings.Contains(response.Header.Get("Location"), "challenge") {
		t.Fatalf("Location = %q carries a challenge token", response.Header.Get("Location"))
	}
	if f.done(id) || cookie(response) != nil {
		t.Fatal("Discord alone finished an account that owes a second factor")
	}
	handoff := handoffCookie(response)
	if handoff == nil || !handoff.HttpOnly || handoff.SameSite != http.SameSiteStrictMode || handoff.Value == "" {
		t.Fatalf("handoff cookie = %+v, want an HttpOnly Strict cookie holding the challenge", handoff)
	}

	_, bare := f.postWith("/api/login/totp", map[string]string{"code": f.code(secret)})
	if bare.StatusCode == http.StatusOK || f.done(id) {
		t.Fatal("a browser without the cookie answered the challenge")
	}
	answer, second := f.postWith("/api/login/totp", map[string]string{"code": f.code(secret)}, handoff)
	if second.StatusCode != http.StatusOK || answer["redirect"] == "" || !f.done(id) {
		t.Fatalf("the second factor did not finish the flow: %d %v", second.StatusCode, answer)
	}
	if cleared := handoffCookie(second); cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("cookie after the answer = %+v, want it expired", cleared)
	}
}

func handoffCookie(response *http.Response) *http.Cookie {
	for _, c := range response.Cookies() {
		if c.Name == "registre-2fa" {
			return c
		}
	}
	return nil
}

func TestSignUpClosedRefusesAnUnknownDiscordAccount(t *testing.T) {
	f := socialFixture(t, social.Profile{Subject: "45", Email: "nobody@facile.studio", EmailVerified: true}, false)
	id := f.authRequest()
	state, held := stateOf(t, f.get("/api/login/social/discord/start?auth_request="+id))

	response := f.get("/api/login/social/discord/callback?code=c&state="+url.QueryEscape(state), held)
	location, _ := url.Parse(response.Header.Get("Location"))
	if location.Query().Get("error") != reasonClosed || location.Query().Get("auth_request") != id {
		t.Fatalf("Location = %q, want the closed reason on the same flow", response.Header.Get("Location"))
	}
}

func linkFixture(t *testing.T, outcome social.Profile) (*fixture, *social.Service) {
	t.Helper()
	var svc *social.Service
	f := newFixtureWith(t, func(service *Service, db *gorm.DB) {
		svc = social.New(db, []social.Provider{fakeProvider{outcome}}, social.Config{Issuer: "https://sso.test", Secure: true})
		service.WithSocial(svc)
		row := schemas.Client{
			ClientID: "place", Subject: "place-subject", Name: "Place", Kind: schemas.KindApplication,
			LoginProviders: schemas.StringList{"discord"},
			RedirectURIs:   schemas.StringList{"https://place.facile.studio/api/auth/callback"},
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("creating the client: %v", err)
		}
	})
	return f, svc
}

func (f *fixture) session(user schemas.User) *http.Cookie {
	f.t.Helper()
	ctx := context.Background()
	id, err := f.users.StartSession(ctx, user.ID, "", time.Now().UTC())
	if err != nil {
		f.t.Fatal(err)
	}
	token, err := accounts.New(f.db).Issue(ctx, id)
	if err != nil {
		f.t.Fatal(err)
	}
	return &http.Cookie{Name: accounts.DevCookieName, Value: token}
}

func TestAFinishedLinkReturnsToTheAppWithTheOutcomeMerged(t *testing.T) {
	f, svc := linkFixture(t, social.Profile{Subject: "77", Name: "Yann"})
	user := f.user("yann@facile.studio", "correct horse battery staple")
	session := f.session(user)

	started, err := svc.Begin(context.Background(), "discord", "", user.ID, "https://place.facile.studio/settings?tab=1")
	if err != nil {
		t.Fatal(err)
	}
	held := &http.Cookie{Name: svc.Cookie(started.State).Name, Value: started.State}
	response := f.get("/api/login/social/discord/callback?code=c&state="+url.QueryEscape(started.State), held, session)
	want := "https://place.facile.studio/settings?connection=linked&tab=1"
	if response.Header.Get("Location") != want {
		t.Fatalf("Location = %q, want %q", response.Header.Get("Location"), want)
	}
}

func TestACanceledLinkReturnsToTheAppAsFailed(t *testing.T) {
	f, svc := linkFixture(t, social.Profile{Subject: "78"})
	user := f.user("yann@facile.studio", "correct horse battery staple")
	started, err := svc.Begin(context.Background(), "discord", "", user.ID, "https://place.facile.studio/settings")
	if err != nil {
		t.Fatal(err)
	}
	held := &http.Cookie{Name: svc.Cookie(started.State).Name, Value: started.State}
	response := f.get("/api/login/social/discord/callback?error=access_denied&state="+url.QueryEscape(started.State), held, f.session(user))
	if response.Header.Get("Location") != "https://place.facile.studio/settings?connection=failed" {
		t.Fatalf("Location = %q", response.Header.Get("Location"))
	}
}

func TestAFinishedLinkWithoutReturnToKeepsTheAccountPage(t *testing.T) {
	f, svc := linkFixture(t, social.Profile{Subject: "79"})
	user := f.user("yann@facile.studio", "correct horse battery staple")
	started, err := svc.Begin(context.Background(), "discord", "", user.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	held := &http.Cookie{Name: svc.Cookie(started.State).Name, Value: started.State}
	response := f.get("/api/login/social/discord/callback?code=c&state="+url.QueryEscape(started.State), held, f.session(user))
	if response.Header.Get("Location") != "/account/security?connection=linked" {
		t.Fatalf("Location = %q", response.Header.Get("Location"))
	}
}

func TestALinkWhoseSessionChangedHandsStillGoesToLogin(t *testing.T) {
	f, svc := linkFixture(t, social.Profile{Subject: "80"})
	owner := f.user("yann@facile.studio", "correct horse battery staple")
	other := f.user("other@facile.studio", "correct horse battery staple")
	started, err := svc.Begin(context.Background(), "discord", "", owner.ID, "https://place.facile.studio/settings")
	if err != nil {
		t.Fatal(err)
	}
	held := &http.Cookie{Name: svc.Cookie(started.State).Name, Value: started.State}
	response := f.get("/api/login/social/discord/callback?code=c&state="+url.QueryEscape(started.State), held, f.session(other))
	if response.Header.Get("Location") != "/login" {
		t.Fatalf("Location = %q, want /login", response.Header.Get("Location"))
	}
}
