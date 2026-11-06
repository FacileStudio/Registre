package social

import (
	"context"
	"errors"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

func addClient(t *testing.T, svc *Service, id string, providers, uris []string, disabled bool) {
	t.Helper()
	row := schemas.Client{
		ClientID: id, Subject: id + "-subject", Name: id, Kind: schemas.KindApplication,
		LoginProviders: providers, RedirectURIs: uris,
	}
	if err := svc.db.Create(&row).Error; err != nil {
		t.Fatalf("creating %s: %v", id, err)
	}
	if disabled {
		svc.db.Model(&row).Update("disabled_at", svc.now())
	}
}

func TestValidReturnAcceptsOnlyARegisteredClientsOrigin(t *testing.T) {
	svc, _ := newService(t, false)
	svc.cfg.Secure = true
	addClient(t, svc, "place", []string{"discord"}, []string{"https://place.facile.studio/api/auth/callback"}, false)
	addClient(t, svc, "other", []string{}, []string{"https://other.facile.studio/cb"}, false)
	addClient(t, svc, "off", []string{"discord"}, []string{"https://off.facile.studio/cb"}, true)

	cases := []struct {
		name, raw string
		ok        bool
	}{
		{"registered origin", "https://place.facile.studio/settings?tab=1", true},
		{"uppercase host", "https://PLACE.facile.studio/settings", true},
		{"other host", "https://evil.example/settings", false},
		{"lookalike subdomain", "https://place.facile.studio.evil.example/", false},
		{"client without the provider", "https://other.facile.studio/x", false},
		{"disabled client", "https://off.facile.studio/x", false},
		{"userinfo", "https://place.facile.studio@evil.example/", false},
		{"userinfo on the right host", "https://user:pw@place.facile.studio/", false},
		{"http in production", "http://place.facile.studio/settings", false},
		{"other port", "https://place.facile.studio:8443/settings", false},
		{"javascript", "javascript:alert(1)", false},
		{"relative", "/account/security", false},
		{"scheme relative", "//place.facile.studio/settings", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		err := svc.ValidReturn(context.Background(), "discord", c.raw)
		if c.ok != (err == nil) || (!c.ok && !errors.Is(err, ErrInvalidReturn)) {
			t.Errorf("%s: ValidReturn(%q) = %v, want ok=%v", c.name, c.raw, err, c.ok)
		}
	}
}

func TestValidReturnAllowsHTTPOnlyWhenTheIssuerIsInsecure(t *testing.T) {
	svc, _ := newService(t, false)
	addClient(t, svc, "place", []string{"discord"}, []string{"http://localhost:5173/cb"}, false)
	svc.cfg.Secure = true
	if err := svc.ValidReturn(context.Background(), "discord", "http://localhost:5173/x"); err == nil {
		t.Fatal("http accepted while Secure")
	}
	svc.cfg.Secure = false
	if err := svc.ValidReturn(context.Background(), "discord", "http://localhost:5173/x"); err != nil {
		t.Fatalf("http refused for an insecure issuer: %v", err)
	}
}

func TestBeginStoresAValidReturnAndRefusesAnInvalidOne(t *testing.T) {
	svc, db := newService(t, false)
	addClient(t, svc, "place", []string{"discord"}, []string{"https://place.facile.studio/cb"}, false)

	if _, err := svc.Begin(context.Background(), "discord", "", "u1", "https://evil.example/"); !errors.Is(err, ErrInvalidReturn) {
		t.Fatalf("Begin with a foreign return_to = %v, want ErrInvalidReturn", err)
	}
	var count int64
	db.Model(&schemas.SocialState{}).Count(&count)
	if count != 0 {
		t.Fatalf("a refused Begin recorded %d states", count)
	}
	started, err := svc.Begin(context.Background(), "discord", "", "u1", "https://place.facile.studio/settings")
	if err != nil {
		t.Fatal(err)
	}
	row, err := svc.Take(context.Background(), started.State, started.State)
	if err != nil || row.ReturnTo != "https://place.facile.studio/settings" {
		t.Fatalf("Take = %+v, %v; want the stored return_to", row, err)
	}
}

func TestReturnURLMergesIntoTheExistingQuery(t *testing.T) {
	svc, _ := newService(t, false)
	addClient(t, svc, "place", []string{"discord"}, []string{"https://place.facile.studio/cb"}, false)
	got := svc.ReturnURL(context.Background(), "discord", "https://place.facile.studio/settings?tab=1", "connection", "linked")
	if got != "https://place.facile.studio/settings?connection=linked&tab=1" {
		t.Fatalf("ReturnURL = %q", got)
	}
	if svc.ReturnURL(context.Background(), "discord", "https://evil.example/", "connection", "linked") != "" {
		t.Fatal("an invalid return_to produced a redirect")
	}
}
