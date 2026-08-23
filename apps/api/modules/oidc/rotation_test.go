package oidc

import (
	"net/http"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/keys"

	jose "github.com/go-jose/go-jose/v4"
)

// lifecycle is a rotation tight enough to drive in one test: a key signs for an
// hour, a replacement is published ten minutes before it takes over, and a
// retired key's public half is kept for two hours.
func lifecycle() keys.Config {
	return keys.Config{RotateEvery: time.Hour, PublishDelay: 10 * time.Minute, Retention: 2 * time.Hour}
}

// TestRotationNeverInvalidatesALiveToken is SPEC §7's requirement, stated as
// the failure it prevents: a token minted before a rotation still verifies
// against the key set published after it. Getting this wrong 401s all eleven
// apps at once, and only an end-to-end assertion catches it — the unit that
// promotes a key is correct in isolation either way.
func TestRotationNeverInvalidatesALiveToken(t *testing.T) {
	fixture := newFixture(t, lifecycle())
	secret := fixture.account("agent-rotate", []string{"courrier.read"}, []string{"courrier"})
	start := fixture.clock.now()

	granted, status := fixture.token("agent-rotate", secret, "")
	if status != http.StatusOK {
		t.Fatalf("token endpoint = %d %s", status, granted.Error)
	}
	before := fixture.activeKID()

	fixture.rotate(start.Add(55 * time.Minute))
	fixture.rotate(start.Add(70 * time.Minute))

	after := fixture.activeKID()
	if after == before {
		t.Fatal("the key did not rotate, so the test proves nothing")
	}
	if _, err := fixture.verify(granted.AccessToken); err != nil {
		t.Fatalf("a token issued before the rotation no longer verifies: %v", err)
	}

	fresh, status := fixture.token("agent-rotate", secret, "")
	if status != http.StatusOK {
		t.Fatalf("token endpoint after rotation = %d %s", status, fresh.Error)
	}
	if _, err := fixture.verify(fresh.AccessToken); err != nil {
		t.Fatalf("a token issued after the rotation does not verify: %v", err)
	}
}

// TestANewKeyIsPublishedBeforeItSigns is the other half: the replacement is in
// the JWKS for the whole publish delay while the old key is still signing, so
// every relying party has had a chance to see it before it matters.
func TestANewKeyIsPublishedBeforeItSigns(t *testing.T) {
	fixture := newFixture(t, lifecycle())
	start := fixture.clock.now()
	original := fixture.activeKID()

	fixture.rotate(start.Add(55 * time.Minute))
	if signing := fixture.activeKID(); signing != original {
		t.Fatalf("the new key started signing at publish time: active is %s", signing)
	}
	published := fixture.jwks()
	if len(published.Keys) != 2 {
		t.Fatalf("the key set holds %d keys during the publish delay, want 2", len(published.Keys))
	}

	fixture.rotate(start.Add(59 * time.Minute))
	if signing := fixture.activeKID(); signing != original {
		t.Fatalf("the new key took over before the publish delay elapsed: active is %s", signing)
	}

	fixture.rotate(start.Add(66 * time.Minute))
	if signing := fixture.activeKID(); signing == original {
		t.Fatal("the new key never took over")
	}
}

// TestARetiredKeyLeavesTheSetOnlyAfterRetention closes the third gap: dropping
// a retired public key while a token it signed is still valid is the same
// outage as never publishing one, arriving later and harder to attribute.
func TestARetiredKeyLeavesTheSetOnlyAfterRetention(t *testing.T) {
	fixture := newFixture(t, lifecycle())
	start := fixture.clock.now()
	original := fixture.activeKID()

	fixture.rotate(start.Add(55 * time.Minute))
	fixture.rotate(start.Add(70 * time.Minute))
	if !holds(fixture.jwks().Keys, original) {
		t.Fatal("the retired key left the set the moment it was retired")
	}

	fixture.rotate(start.Add(70*time.Minute + 119*time.Minute))
	if !holds(fixture.jwks().Keys, original) {
		t.Fatal("the retired key left the set before its retention elapsed")
	}

	fixture.rotate(start.Add(70*time.Minute + 121*time.Minute))
	if holds(fixture.jwks().Keys, original) {
		t.Fatal("the retired key is still published past its retention")
	}
}

// holds reports whether a key set carries a kid.
func holds(published []jose.JSONWebKey, kid string) bool {
	for _, key := range published {
		if key.KeyID == kid {
			return true
		}
	}
	return false
}
