package oidc

import (
	"net/http"
	"testing"
)

// TestOnlyTheClientAToken WasIssuedToCanRevokeIt is RFC 7009 §2.1 stated as the
// failure it prevents: without the check, any principal holding valid
// credentials could sign every other principal out of the suite.
func TestOnlyTheClientATokenWasIssuedToCanRevokeIt(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	mine := fixture.account("agent-mine", []string{"courrier.read"}, []string{"courrier"})
	theirs := fixture.account("agent-theirs", []string{"courrier.read"}, []string{"courrier"})
	app := fixture.account("courrier", []string{"courrier.read"}, []string{"courrier"})

	granted, status := fixture.token("agent-mine", mine, "")
	if status != http.StatusOK {
		t.Fatalf("token endpoint = %d %s", status, granted.Error)
	}

	if status := fixture.revoke("agent-theirs", theirs, granted.AccessToken); status != http.StatusOK {
		t.Fatalf("revocation by another client = %d, want 200 — RFC 7009 answers 200 either way", status)
	}
	answer, _ := fixture.introspect("courrier", app, granted.AccessToken)
	if answer["active"] != true {
		t.Fatal("another client's revocation took the token away")
	}

	if status := fixture.revoke("agent-mine", mine, granted.AccessToken); status != http.StatusOK {
		t.Fatalf("revoking my own token = %d", status)
	}
	answer, _ = fixture.introspect("courrier", app, granted.AccessToken)
	if answer["active"] != false {
		t.Fatal("a revoked token still introspects as active")
	}
}

// TestIntrospectionAnswersOnlyAnAudience keeps the register from becoming a
// directory of who holds what. An authenticated client that is neither the
// issuer nor an audience of a token gets the same answer as for a token that
// never existed.
func TestIntrospectionAnswersOnlyAnAudience(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	agent := fixture.account("agent-scoped", []string{"courrier.read"}, []string{"courrier"})
	stranger := fixture.account("casier", []string{"casier.read"}, []string{"casier"})

	granted, status := fixture.token("agent-scoped", agent, "")
	if status != http.StatusOK {
		t.Fatalf("token endpoint = %d %s", status, granted.Error)
	}

	answer, _ := fixture.introspect("casier", stranger, granted.AccessToken)
	if answer["active"] != false {
		t.Fatalf("an app that is not an audience learned about the token: %v", answer)
	}
	if answer["sub"] != nil {
		t.Errorf("the inactive answer leaked a subject: %v", answer["sub"])
	}
}

// TestIntrospectionReportsWhatTheRegisterGranted is the audience's side of the
// same endpoint: an app that the token names may read its subject, scopes and
// expiry, which is how it authorizes the caller.
func TestIntrospectionReportsWhatTheRegisterGranted(t *testing.T) {
	fixture := newFixture(t, defaultKeyConfig())
	agent := fixture.account("agent-known", []string{"courrier.read"}, []string{"courrier"})
	app := fixture.account("courrier", []string{"courrier.read"}, []string{"courrier"})

	granted, status := fixture.token("agent-known", agent, "")
	if status != http.StatusOK {
		t.Fatalf("token endpoint = %d %s", status, granted.Error)
	}
	answer, _ := fixture.introspect("courrier", app, granted.AccessToken)
	if answer["active"] != true {
		t.Fatalf("the audience could not introspect its own token: %v", answer)
	}
	if answer["client_id"] != "agent-known" {
		t.Errorf("client_id = %v, want agent-known", answer["client_id"])
	}
	if answer["scope"] != "courrier.read" {
		t.Errorf("scope = %v, want courrier.read", answer["scope"])
	}
	if answer["iss"] != fixture.issuer {
		t.Errorf("iss = %v, want %s", answer["iss"], fixture.issuer)
	}
}
