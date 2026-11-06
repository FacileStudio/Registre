package spaces

import (
	"net/http"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/accounts"
)

// checkout is the body the page sends to buy Pro monthly.
func checkout() map[string]string { return map[string]string{"lookup_key": "plan_pro_month"} }

// TestOwnersSeeTheCatalogMembersOnlyThePlan pins what each rank reads, and
// that an unreachable Ardoise degrades to "unavailable" instead of failing.
func TestOwnersSeeTheCatalogMembersOnlyThePlan(t *testing.T) {
	w := newWorld(t)
	status, owner := w.call(w.on, http.MethodGet, "/billing", w.yann, nil)
	if status != http.StatusOK || owner["owner"] != true || owner["available"] != true || owner["catalog"] == nil {
		t.Errorf("owner view = %d %v, want the plan and the catalog", status, owner)
	}
	_, member := w.call(w.on, http.MethodGet, "/billing", w.bob, nil)
	if member["owner"] != false || member["plan"] == nil || member["catalog"] != nil {
		t.Errorf("member view = %v, want the plan without the catalog", member)
	}
	w.fake.set(func() { w.fake.down = true })
	_, down := w.call(w.on, http.MethodGet, "/billing", w.yann, nil)
	if down["enabled"] != true || down["available"] != false {
		t.Errorf("view with Ardoise down = %v, want enabled and unavailable", down)
	}
	_, off := w.call(w.off, http.MethodGet, "/billing", w.yann, nil)
	if off["enabled"] != false {
		t.Errorf("view with billing off = %v, want enabled false", off)
	}
}

// TestOnlyAnOwnerStartsACheckout is the server-side owner check, the CSRF
// guard and the server-built checkout body.
func TestOnlyAnOwnerStartsACheckout(t *testing.T) {
	w := newWorld(t)
	cases := []struct {
		name, path, cookie string
		body               any
		want               int
	}{
		{"an admin checks out", "/billing/checkout", w.bob, checkout(), http.StatusForbidden},
		{"an admin opens the portal", "/billing/portal", w.bob, map[string]any{}, http.StatusForbidden},
		{"no session", "/billing/checkout", "", checkout(), http.StatusUnauthorized},
		{"a malformed lookup key", "/billing/checkout", w.yann, map[string]string{"lookup_key": "../x"}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		if status, _ := w.call(w.on, http.MethodPost, tc.path, tc.cookie, tc.body); status != tc.want {
			t.Errorf("%s: answered %d, want %d", tc.name, status, tc.want)
		}
	}
	forged := w.request(http.MethodPost, "/billing/checkout", w.yann, checkout())
	forged.Header.Del(accounts.CSRFHeader)
	if status, _ := serve(w.on, forged); status != http.StatusForbidden {
		t.Errorf("no CSRF header: answered %d, want 403", status)
	}
	if status, _ := w.call(w.off, http.MethodPost, "/billing/checkout", w.yann, checkout()); status != http.StatusNotFound {
		t.Errorf("billing off: answered %d, want 404", status)
	}
	if len(w.fake.checkouts) != 0 {
		t.Fatalf("a refused checkout still reached Ardoise: %+v", w.fake.checkouts)
	}
	status, out := w.call(w.on, http.MethodPost, "/billing/checkout", w.yann, checkout())
	if status != http.StatusOK || out["url"] != "https://checkout.stripe.com/c/pay/cs_test" {
		t.Fatalf("owner checkout = %d %v", status, out)
	}
	sent := w.fake.checkouts[0]
	if sent.OwnerEmail != "yann@facile.studio" || sent.SpaceID != w.space ||
		!strings.HasPrefix(sent.SuccessURL, "https://sso.test/spaces/"+w.space) {
		t.Errorf("checkout body = %+v, want it built from server state", sent)
	}
}

// TestOnlyStripeOriginsAreHandedOn keeps a compromised Ardoise from turning
// the plan section into an open redirect.
func TestOnlyStripeOriginsAreHandedOn(t *testing.T) {
	w := newWorld(t)
	for _, target := range []string{
		"https://evil.example/pay", "http://checkout.stripe.com/x", "https://checkout.stripe.com.evil.example/x",
		"https://user@checkout.stripe.com/x", "javascript:alert(1)", "",
	} {
		w.fake.set(func() { w.fake.redirect = target })
		if status, _ := w.call(w.on, http.MethodPost, "/billing/checkout", w.yann, checkout()); status != http.StatusBadGateway {
			t.Errorf("checkout redirect %q answered %d, want 502", target, status)
		}
	}
	w.fake.set(func() { w.fake.redirect = "https://billing.stripe.com/p/session/x" })
	status, out := w.call(w.on, http.MethodPost, "/billing/portal", w.yann, map[string]any{"owner_email": "mallory@example.com"})
	if status != http.StatusOK || out["url"] != w.fake.redirect {
		t.Errorf("owner portal = %d %v", status, out)
	}
	w.fake.set(func() {
		last := w.fake.portals[len(w.fake.portals)-1]
		if last["owner_email"] != "yann@facile.studio" || last["space_id"] != w.space {
			t.Errorf("Ardoise was sent %v, want the signed-in owner's address from the session", last)
		}
	})
}
