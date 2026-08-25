package clients

import (
	"context"
	"testing"
)

// TestEnsureSelfIsIdempotentAndHoldsNoSecret pins the door registre uses to
// let a human sign in to itself.
//
// It runs at every boot, so a second call that minted a new subject would
// re-key the client on every restart, and a row that carried a secret would
// mean a browser was expected to hold one. Both are silent failures: the first
// orphans nothing visible until somebody audits subjects, and the second only
// shows up as a login that refuses for no stated reason.
func TestEnsureSelfIsIdempotentAndHoldsNoSecret(t *testing.T) {
	register, _ := register(t)
	const redirect = "https://sso.facile.studio/device"

	if err := register.EnsureSelf(context.Background(), redirect); err != nil {
		t.Fatalf("the first registration: %v", err)
	}
	first, err := register.ByClientID(context.Background(), SelfClientID)
	if err != nil {
		t.Fatalf("reading the row back: %v", err)
	}
	if err := register.EnsureSelf(context.Background(), redirect); err != nil {
		t.Fatalf("the second registration: %v", err)
	}
	second, err := register.ByClientID(context.Background(), SelfClientID)
	if err != nil {
		t.Fatalf("reading the row back: %v", err)
	}

	switch {
	case first.Subject != second.Subject:
		t.Errorf("the subject moved from %s to %s across a restart", first.Subject, second.Subject)
	case !second.Public:
		t.Error("registre's own client is not public, so a browser is expected to hold a secret")
	case second.SecretHash != "":
		t.Error("registre's own client carries a secret hash")
	case !second.RedirectURIs.Contains(redirect):
		t.Errorf("redirect_uris = %v, want the verification page", second.RedirectURIs)
	case second.Managed:
		t.Error("the row is marked seed-managed, so the seed will report it as dropped")
	}
}
