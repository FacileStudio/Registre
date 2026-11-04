package signup

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/FacileStudio/Registre/apps/api/schemas"

	altcha "github.com/altcha-org/altcha-lib-go"
)

// TestStartAnswersTheSameForKnownAndUnknownAddresses is the enumeration
// guard: the same error (none), the same database work, a mail either way,
// and comparable timing.
func TestStartAnswersTheSameForKnownAndUnknownAddresses(t *testing.T) {
	w := newWorld(t, map[string]bool{})
	ctx := context.Background()
	if err := w.db.Create(&schemas.User{ID: "known", Email: "known@example.com", Locale: "fr"}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	var known, unknown []time.Duration
	for i := 0; i < 8; i++ {
		for _, email := range []string{"known@example.com", "stranger@example.com"} {
			proof := solve(t, w.svc)
			started := time.Now()
			err := w.svc.Start(ctx, Request{Email: email, Proof: proof, Locale: "en"})
			took := time.Since(started)
			if err != nil {
				t.Fatalf("Start(%s) = %v, want nil", email, err)
			}
			if email == "known@example.com" {
				known = append(known, took)
				if !strings.Contains(w.inbox.last(t).subject, "déjà") {
					t.Fatalf("the known address got %q, want the French already-have-an-account mail", w.inbox.last(t).subject)
				}
			} else {
				unknown = append(unknown, took)
				if !strings.Contains(w.inbox.last(t).html, "/signup/finish?token=") {
					t.Fatal("the unknown address got no sign-up link")
				}
			}
		}
	}
	var rowsKnown, rowsUnknown int64
	w.db.Model(&schemas.SignupChallenge{}).Where("email = ?", "known@example.com").Count(&rowsKnown)
	w.db.Model(&schemas.SignupChallenge{}).Where("email = ?", "stranger@example.com").Count(&rowsUnknown)
	if rowsKnown != rowsUnknown {
		t.Fatalf("rows written: known %d, unknown %d; both branches must do the same work", rowsKnown, rowsUnknown)
	}
	a, b := median(known), median(unknown)
	if diff := (a - b).Abs(); diff > 20*time.Millisecond && diff > max(a, b)/2 {
		t.Fatalf("median timing known %s, unknown %s: too far apart", a, b)
	}
}

func median(samples []time.Duration) time.Duration {
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return samples[len(samples)/2]
}

// TestAProofIsSpentOnce refuses a replayed ALTCHA solution.
func TestAProofIsSpentOnce(t *testing.T) {
	w := newWorld(t, map[string]bool{})
	proof := solve(t, w.svc)
	ctx := context.Background()
	if err := w.svc.Start(ctx, Request{Email: "a@example.com", Proof: proof}); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if err := w.svc.Start(ctx, Request{Email: "b@example.com", Proof: proof}); !errors.Is(err, ErrProof) {
		t.Fatalf("replayed proof = %v, want ErrProof", err)
	}
}

// TestForgedAndExpiredProofsAreRefused covers a wrong answer, a challenge
// signed with another key, a missing proof and an expired challenge.
func TestForgedAndExpiredProofsAreRefused(t *testing.T) {
	w := newWorld(t, map[string]bool{})
	ctx := context.Background()
	var payload altcha.Payload
	raw, _ := base64.StdEncoding.DecodeString(solve(t, w.svc))
	_ = json.Unmarshal(raw, &payload)
	payload.Number++
	wrong, _ := json.Marshal(payload)

	expiredAt := time.Now().Add(-time.Minute)
	stale, _ := altcha.CreateChallenge(altcha.ChallengeOptions{HMACKey: testKey, MaxNumber: 10, Expires: &expiredAt})
	foreign, _ := altcha.CreateChallenge(altcha.ChallengeOptions{HMACKey: "another-key-of-at-least-thirty-two-bytes", MaxNumber: 10})
	for name, proof := range map[string]string{
		"wrong number":   base64.StdEncoding.EncodeToString(wrong),
		"missing":        "",
		"not base64":     "%%%",
		"expired":        solved(t, stale),
		"foreign signer": solved(t, foreign),
	} {
		if err := w.svc.Start(ctx, Request{Email: "a@example.com", Proof: proof}); !errors.Is(err, ErrProof) {
			t.Errorf("%s proof = %v, want ErrProof", name, err)
		}
	}
}

// solved answers a challenge minted outside the service.
func solved(t *testing.T, challenge altcha.Challenge) string {
	t.Helper()
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

// TestAMalformedAddressIsRefusedAfterTheProof decides from the string.
func TestAMalformedAddressIsRefusedAfterTheProof(t *testing.T) {
	w := newWorld(t, map[string]bool{})
	err := w.svc.Start(context.Background(), Request{Email: "not an address", Proof: solve(t, w.svc)})
	if !errors.Is(err, ErrEmail) {
		t.Fatalf("Start = %v, want ErrEmail", err)
	}
}
