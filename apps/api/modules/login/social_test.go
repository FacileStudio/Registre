package login

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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
	if err != nil || location.Path != "/login" || location.Query().Get("challenge") == "" {
		t.Fatalf("Location = %q, want /login with a challenge", response.Header.Get("Location"))
	}
	if f.done(id) || cookie(response) != nil {
		t.Fatal("Discord alone finished an account that owes a second factor")
	}
	answer, second := f.second(location.Query().Get("challenge"), f.code(secret))
	if second.StatusCode != http.StatusOK || answer["redirect"] == "" || !f.done(id) {
		t.Fatalf("the second factor did not finish the flow: %d %v", second.StatusCode, answer)
	}
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
