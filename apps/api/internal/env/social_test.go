package env

import "testing"

func TestSocialNeedsBothDiscordValuesOrNeither(t *testing.T) {
	t.Setenv("DISCORD_CLIENT_ID", "")
	t.Setenv("DISCORD_CLIENT_SECRET", "")
	off, err := LoadSocial()
	if err != nil || off.DiscordEnabled() {
		t.Fatalf("both empty must load as disabled, got %+v, %v", off, err)
	}
	t.Setenv("DISCORD_CLIENT_ID", "123")
	if _, err := LoadSocial(); err == nil {
		t.Fatal("an id without a secret must fail the boot")
	}
	t.Setenv("DISCORD_CLIENT_SECRET", "shh")
	on, err := LoadSocial()
	if err != nil || !on.DiscordEnabled() {
		t.Fatalf("both set must load as enabled, got %+v, %v", on, err)
	}
}
