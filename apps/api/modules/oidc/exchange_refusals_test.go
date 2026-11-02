package oidc

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

type refusal struct {
	name   string
	extra  url.Values
	status int
	code   string
}

func TestEveryBadRequestIsRefusedWithItsClassOfError(t *testing.T) {
	world := newExchangeWorld(t)
	cases := []refusal{
		{"no audience", url.Values{"audience": nil}, http.StatusBadRequest, "invalid_target"},
		{"two audiences", url.Values{"audience": {"courrier", "agenda"}}, http.StatusBadRequest, "invalid_target"},
		{"an app the caller may not reach", url.Values{"audience": {"opus"}}, http.StatusBadRequest, "invalid_target"},
		{"the caller itself", url.Values{"audience": {"bulle"}}, http.StatusBadRequest, "invalid_target"},
		{"a resource indicator", url.Values{"resource": {"https://courrier.facile.studio"}}, http.StatusBadRequest, "invalid_target"},
		{"a scope not in the allowance", url.Values{"scope": {"courrier.write"}}, http.StatusBadRequest, "invalid_scope"},
		{"a scope of another app", url.Values{"scope": {"agenda.read"}}, http.StatusBadRequest, "invalid_scope"},
		{"an identity scope", url.Values{"scope": {"openid"}}, http.StatusBadRequest, "invalid_scope"},
		{"a refresh token asked for", url.Values{"requested_token_type": {refreshTokenURN}}, http.StatusBadRequest, "invalid_request"},
		{"an id token asked for", url.Values{"requested_token_type": {idTokenURN}}, http.StatusBadRequest, "invalid_request"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { world.expectRefusal(t, world.access, c) })
	}
}

func (w *exchangeWorld) expectRefusal(t *testing.T, subject string, c refusal) {
	t.Helper()
	body, status := w.exchange(subject, c.extra)
	if status != c.status || body["error"] != c.code {
		t.Errorf("%s: got %d %v, want %d %s", c.name, status, body, c.status, c.code)
	}
	if _, issued := body["access_token"]; issued {
		t.Errorf("%s: a token was issued alongside the refusal", c.name)
	}
}

func TestAnActorTokenIsRefused(t *testing.T) {
	world := newExchangeWorld(t)
	world.expectRefusal(t, world.access, refusal{
		"an actor token", url.Values{"actor_token": {world.access}, "actor_token_type": {accessTokenURN}},
		http.StatusBadRequest, "invalid_request",
	})
}

func TestTheSubjectTokenIsRefusedWhenItIsNotLive(t *testing.T) {
	world := newExchangeWorld(t)
	jti := world.subjectJTI()
	revoked := refusal{"a revoked subject", nil, http.StatusBadRequest, "invalid_grant"}
	world.updateToken(jti, "revoked_at", world.clock.now())
	first, _ := world.exchange(world.access, nil)
	world.expectRefusal(t, world.access, revoked)

	world.updateToken(jti, "revoked_at", nil)
	world.updateToken(jti, "expires_at", world.clock.now().Add(-time.Minute))
	second, _ := world.exchange(world.access, nil)
	world.expectRefusal(t, world.access, refusal{"an expired subject", nil, http.StatusBadRequest, "invalid_grant"})
	if first["error_description"] != second["error_description"] {
		t.Errorf("two different subject failures answer differently: %q vs %q",
			first["error_description"], second["error_description"])
	}
}

func TestTheSubjectTokenIsRefusedWhenItIsNotTheCallers(t *testing.T) {
	world := newExchangeWorld(t)
	world.updateToken(world.subjectJTI(), "client_id", "agenda")
	world.expectRefusal(t, world.access, refusal{"issued to another client", nil, http.StatusBadRequest, "invalid_grant"})
}

func TestTheSubjectTokenIsRefusedWhenAudienceExcludesTheCaller(t *testing.T) {
	world := newExchangeWorld(t)
	world.updateToken(world.subjectJTI(), "audiences", schemas.StringList{"agenda"})
	world.expectRefusal(t, world.access, refusal{"addressed to another app", nil, http.StatusBadRequest, "invalid_grant"})
}

func TestADelegatedTokenCannotBeDelegatedAgain(t *testing.T) {
	world := newExchangeWorld(t)
	granted, _ := world.exchange(world.access, nil)
	delegated, _ := granted["access_token"].(string)
	claims, _ := world.verify(delegated)
	jti, _ := claims["jti"].(string)
	world.updateToken(jti, "audiences", schemas.StringList{"bulle"})
	world.expectRefusal(t, delegated, refusal{"a second hop", nil, http.StatusBadRequest, "invalid_grant"})
}

func TestADisabledHumanCannotBeActedFor(t *testing.T) {
	world := newExchangeWorld(t)
	if _, err := world.users.Disable(context.Background(), world.userID, world.clock.now()); err != nil {
		t.Fatalf("disabling the human: %v", err)
	}
	world.expectRefusal(t, world.access, refusal{"a disabled human", nil, http.StatusBadRequest, "invalid_grant"})
}

func TestOnlyAnAccessTokenIsAcceptedAsTheSubject(t *testing.T) {
	world := newExchangeWorld(t)
	body, status := world.post("/oauth/token", "bulle", world.secret, url.Values{
		"grant_type": {exchangeGrantURN}, "subject_token": {world.idToken},
		"subject_token_type": {idTokenURN}, "audience": {"courrier"},
	})
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Errorf("an id token as subject: %d %v", status, body)
	}
}

func TestAServiceAccountTokenIsNotAHumansToken(t *testing.T) {
	world := newExchangeWorld(t)
	secret := world.account("worker", []string{"courrier.read"}, []string{"bulle"})
	machine, _ := world.token("worker", secret, "")
	world.expectRefusal(t, machine.AccessToken, refusal{"a machine as subject", nil, http.StatusBadRequest, "invalid_grant"})
}

func TestTheCallerMustAuthenticateAndHoldTheGrant(t *testing.T) {
	world := newExchangeWorld(t)
	body, status := world.exchangeAs("bulle", "not-the-secret", world.access, nil)
	if status != http.StatusUnauthorized || body["error"] != "invalid_client" {
		t.Errorf("a wrong secret: %d %v", status, body)
	}
	other := world.application("agenda-ui", redirectOf("agenda-ui"))
	body, status = world.exchangeAs("agenda-ui", other, world.access, nil)
	if status != http.StatusBadRequest || body["error"] != "unauthorized_client" {
		t.Errorf("a client without the grant: %d %v", status, body)
	}
}

func TestAPublicClientCannotExchange(t *testing.T) {
	world := newExchangeWorld(t)
	world.publicClient("facile-cli", []string{exchangeGrantURN}, []string{"openid"})
	world.allowExchange("facile-cli", schemas.ExchangeGrants{{Audience: "courrier", Scopes: []string{"courrier.read"}}})
	body, status := world.postForm("/oauth/token", url.Values{
		"client_id": {"facile-cli"}, "grant_type": {exchangeGrantURN}, "subject_token": {world.access},
		"subject_token_type": {accessTokenURN}, "audience": {"courrier"},
	})
	if status < http.StatusBadRequest || body["error"] == nil {
		t.Errorf("a public client exchanged: %d %v", status, body)
	}
	if _, issued := body["access_token"]; issued {
		t.Error("a public client was issued a delegated token")
	}
}

func TestAJunkSubjectTokenIsRefusedAndIssuesNothing(t *testing.T) {
	world := newExchangeWorld(t)
	for _, junk := range []string{"not-a-token", world.access + "x", world.access[:len(world.access)/2]} {
		body, status := world.exchange(junk, nil)
		if status != http.StatusBadRequest || body["error"] == nil {
			t.Errorf("junk subject %q: %d %v", junk[:6], status, body)
		}
		if _, issued := body["access_token"]; issued {
			t.Errorf("junk subject %q was issued a token", junk[:6])
		}
	}
}
