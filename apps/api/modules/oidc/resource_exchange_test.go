package oidc

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

func TestAnExchangeForAResourceIsAddressedToTheResource(t *testing.T) {
	world := newExchangeWorld(t)
	world.declareResource("courrier", courrierMCP, "courrier.read", "courrier.write")

	for name, extra := range map[string]url.Values{
		"resource alone":            {"audience": nil, "resource": {courrierMCP}},
		"resource and its app":      {"audience": {"courrier"}, "resource": {courrierMCP}},
		"resource and the same URI": {"audience": {courrierMCP}, "resource": {courrierMCP}},
	} {
		t.Run(name, func(t *testing.T) {
			granted, status := world.exchange(world.access, extra)
			if status != http.StatusOK {
				t.Fatalf("exchange = %d: %v", status, granted)
			}
			claims := mustClaims(t, world.fixture, granted["access_token"])
			if only(claims["aud"]) != courrierMCP {
				t.Errorf("aud = %v, want only %s", claims["aud"], courrierMCP)
			}
			if claims["scope"] != "courrier.read" {
				t.Errorf("scope = %v, want bulle's courrier allowance, courrier.read", claims["scope"])
			}
			actor, _ := claims["act"].(map[string]any)
			if actor["sub"] != "bulle" {
				t.Errorf("act = %v, want bulle", claims["act"])
			}
		})
	}
}

func TestAnExchangeForAResourceKeepsTheAppsRules(t *testing.T) {
	world := newExchangeWorld(t)
	world.declareResource("courrier", courrierMCP, "courrier.read", "courrier.write")
	world.declareResource("opus", "https://opus.test/mcp", "opus.read")
	cases := map[string]url.Values{
		"an undeclared resource":        {"resource": {"https://nowhere.test/mcp"}},
		"an app bulle has no grant for": {"audience": nil, "resource": {"https://opus.test/mcp"}},
		"a mismatched audience":         {"audience": {"agenda"}, "resource": {courrierMCP}},
		"a scope beyond the allowance":  {"resource": {courrierMCP}, "scope": {"courrier.write"}},
		"two resources":                 {"resource": {courrierMCP, "https://opus.test/mcp"}},
		"an audience of a stray URL":    {"audience": {"https://nowhere.test/mcp"}},
	}
	for name, extra := range cases {
		t.Run(name, func(t *testing.T) {
			granted, status := world.exchange(world.access, extra)
			if status == http.StatusOK {
				t.Fatalf("exchange for %s was granted: %v", name, granted)
			}
		})
	}
}

func TestAnExchangeForARestrictedAppsResourceNeedsAccess(t *testing.T) {
	world := newExchangeWorld(t)
	world.declareResource("courrier", courrierMCP, "courrier.read")
	if err := world.db.Model(&schemas.Client{}).Where("client_id = ?", "courrier").
		Update("access_policy", schemas.AccessAssigned).Error; err != nil {
		t.Fatalf("restricting courrier: %v", err)
	}
	granted, status := world.exchange(world.access, url.Values{"audience": nil, "resource": {courrierMCP}})
	if status != http.StatusBadRequest || granted["error"] != "access_denied" {
		t.Fatalf("exchange for a restricted app's resource = %d %v, want access_denied", status, granted)
	}
}
