package seed

import (
	"strings"
	"testing"
)

func TestLoginProvidersAreValidatedAgainstTheKnownOnes(t *testing.T) {
	t.Setenv("APP_SECRET", "set")
	doc := func(provider string) string {
		return "applications:\n  - client_id: app\n    secret_env: APP_SECRET\n" +
			"    redirect_uris: [https://app.test/cb]\n    login_providers: [" + provider + "]\n"
	}
	file, err := Load(write(t, doc("discord")))
	if err != nil {
		t.Fatalf("discord is a known provider: %v", err)
	}
	if got := file.Applications[0].LoginProviders; len(got) != 1 || got[0] != "discord" {
		t.Fatalf("login_providers = %v, want [discord]", got)
	}
	_, err = Load(write(t, doc("myspace")))
	if err == nil || !strings.Contains(err.Error(), "myspace") {
		t.Fatalf("an unknown provider must fail the boot and name itself, got %v", err)
	}
}
