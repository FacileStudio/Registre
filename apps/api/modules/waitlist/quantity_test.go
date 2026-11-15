package waitlist

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func pastilleBody(email, quantity, altchaProof string) string {
	return fmt.Sprintf(`{"email":%q,"product":"pastille","locale":"fr","consent":"2026-11-14","quantity":%s,"altcha":%q}`,
		email, quantity, altchaProof)
}

func TestJoinStoresAnOptionalPastilleQuantity(t *testing.T) {
	w := build(t, roomy())
	if rec := send(w, http.MethodPost, "/api/waitlist", scribe, pastilleBody("three@example.com", "3", proof(t, w))); rec.Code != http.StatusAccepted {
		t.Fatalf("quantity 3: %d %s", rec.Code, rec.Body)
	}
	if rec := send(w, http.MethodPost, "/api/waitlist", scribe, joinBodyFor("none@example.com", "pastille", proof(t, w))); rec.Code != http.StatusAccepted {
		t.Fatalf("no quantity: %d %s", rec.Code, rec.Body)
	}
	got := map[string]*int16{}
	for _, row := range rows(t, w) {
		got[row.Email] = row.Quantity
	}
	if q := got["three@example.com"]; q == nil || *q != 3 {
		t.Fatalf("stored quantity %v, want 3", q)
	}
	if q, ok := got["none@example.com"]; !ok || q != nil {
		t.Fatalf("missing quantity stored as %v (present %v), want NULL", q, ok)
	}
}

func TestJoinRefusesABadQuantity(t *testing.T) {
	w := build(t, roomy())
	cases := map[string]string{
		"zero":      pastilleBody("p@example.com", "0", proof(t, w)),
		"eleven":    pastilleBody("p@example.com", "11", proof(t, w)),
		"fraction":  pastilleBody("p@example.com", "2.5", proof(t, w)),
		"on scribe": strings.Replace(pastilleBody("p@example.com", "2", proof(t, w)), `"pastille"`, `"scribe"`, 1),
	}
	for name, body := range cases {
		rec := send(w, http.MethodPost, "/api/waitlist", scribe, body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"invalid_quantity"`) {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
	if rec := send(w, http.MethodPost, "/api/waitlist", scribe, pastilleBody("p@example.com", `"3"`, proof(t, w))); rec.Code != http.StatusBadRequest {
		t.Errorf("string quantity: %d %s", rec.Code, rec.Body)
	}
	if len(rows(t, w)) != 0 {
		t.Fatal("a refused quantity wrote a row")
	}
}
