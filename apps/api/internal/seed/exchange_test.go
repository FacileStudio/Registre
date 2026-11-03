package seed

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/schemas"
)

const exchangeApps = `applications:
  - client_id: bulle-id
    name: Bulle
    secret_env: EXCHANGE_SECRET
    redirect_uris: [https://bulle.example/cb]
%s
  - client_id: courrier-id
    name: Courrier
    secret_env: EXCHANGE_SECRET
    redirect_uris: [https://courrier.example/cb]
`

func exchangeDocument(block string) string {
	return strings.Replace(exchangeApps, "%s", block, 1)
}

func TestATokenExchangeBlockLoadsAndGrantsTheExchange(t *testing.T) {
	t.Setenv("EXCHANGE_SECRET", "set")
	file, err := Load(write(t, exchangeDocument(
		"    token_exchange:\n      - audience: courrier-id\n        scopes: [courrier.read]")))
	if err != nil {
		t.Fatalf("a valid block was refused: %v", err)
	}
	bulle := file.Applications[0]
	grants, err := bulle.grants()
	if err != nil || !contains(grants, exchangeGrantType) {
		t.Errorf("grants = %v (%v), want the exchange grant added", grants, err)
	}
	want := schemas.ExchangeGrants{{Audience: "courrier-id", Scopes: []string{"courrier.read"}}}
	if got := bulle.exchangeGrants(); len(got) != 1 || got[0].Audience != want[0].Audience || got[0].Scopes[0] != "courrier.read" {
		t.Errorf("exchangeGrants() = %v, want %v", got, want)
	}
	if other := file.Applications[1]; len(other.exchangeGrants()) != 0 || contains(mustGrants(t, other), exchangeGrantType) {
		t.Error("an application declaring no token_exchange gained the grant")
	}
}

func mustGrants(t *testing.T, a Application) []string {
	t.Helper()
	grants, err := a.grants()
	if err != nil {
		t.Fatalf("grants: %v", err)
	}
	return grants
}

func TestATokenExchangeBlockThatCouldNotWorkFailsTheBoot(t *testing.T) {
	cases := map[string]string{
		"an unknown audience":      "      - audience: nobody\n        scopes: [courrier.read]",
		"itself":                   "      - audience: bulle-id\n        scopes: [bulle.read]",
		"no audience":              "      - scopes: [courrier.read]",
		"no scopes":                "      - audience: courrier-id",
		"a scope of another app":   "      - audience: courrier-id\n        scopes: [agenda.read]",
		"a scope with no action":   "      - audience: courrier-id\n        scopes: [courrier]",
		"an unknown action":        "      - audience: courrier-id\n        scopes: [courrier.admin]",
		"an upper case scope":      "      - audience: courrier-id\n        scopes: [Courrier.read]",
		"the same audience twice":  "      - audience: courrier-id\n        scopes: [courrier.read]\n      - audience: courrier-id\n        scopes: [courrier.write]",
		"a typo in the entry keys": "      - audiance: courrier-id\n        scopes: [courrier.read]",
	}
	for name, entry := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("EXCHANGE_SECRET", "set")
			if _, err := Load(write(t, exchangeDocument("    token_exchange:\n"+entry))); err == nil {
				t.Fatalf("%s was accepted", name)
			}
		})
	}
}

func TestAPublicClientCannotDeclareAnExchange(t *testing.T) {
	t.Setenv("EXCHANGE_SECRET", "set")
	body := `applications:
  - client_id: cli
    name: Cli
    public: true
    grant_types: [device_code]
    token_exchange:
      - audience: courrier-id
        scopes: [courrier.read]
  - client_id: courrier-id
    name: Courrier
    secret_env: EXCHANGE_SECRET
    redirect_uris: [https://courrier.example/cb]
`
	if _, err := Load(write(t, body)); err == nil {
		t.Fatal("a public client was allowed to declare an exchange")
	}
}

func TestApplyWritesTheAllowanceAndAReapplyRewritesIt(t *testing.T) {
	t.Setenv("EXCHANGE_SECRET", "set")
	register, _ := register(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)
	with, err := Load(write(t, exchangeDocument(
		"    token_exchange:\n      - audience: courrier-id\n        scopes: [courrier.read]")))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := Apply(ctx, register, with, logger); err != nil {
		t.Fatalf("apply: %v", err)
	}
	row, err := register.ByClientID(ctx, "bulle-id")
	if err != nil || !row.GrantTypes.Contains(exchangeGrantType) || len(row.TokenExchange) != 1 {
		t.Fatalf("stored row = %+v (%v)", row, err)
	}
	without, err := Load(write(t, exchangeDocument("")))
	if err != nil {
		t.Fatalf("load without: %v", err)
	}
	if err := Apply(ctx, register, without, logger); err != nil {
		t.Fatalf("reapply: %v", err)
	}
	row, _ = register.ByClientID(ctx, "bulle-id")
	if row.GrantTypes.Contains(exchangeGrantType) || len(row.TokenExchange) != 0 {
		t.Errorf("dropping the block left the allowance in place: %+v", row)
	}
}

func TestTheProductionSeedLetsBulleReadAndWriteSixApps(t *testing.T) {
	file := loadProductionSeed(t)
	var bulle Application
	for _, app := range file.Applications {
		if app.ClientID == "bulle" {
			bulle = app
		}
	}
	want := map[string]string{
		"tniN7PWx8mSOCwVAAvYpBexYnMFWAM5flt19rNl0": "courrier",
		"vvVAmcun8BFuAxMdNrp6lhqs2anzvz3vnzkeZh3U": "agenda",
		"aHKmbSdWbJKqUMdTjIqFz2h4YtKYkvMiTB39zr4D": "opus",
		"5TVdL3zCbhhh7XCs3MvaIr5nqoV2ZIMLeEdjnVYu": "sablier",
		"9U40zs8fnTkpKpR6MMobIOXk0OkvotC7vd1NECGH": "nuage",
		"mycelium": "mycelium",
	}
	if len(bulle.TokenExchange) != len(want) {
		t.Fatalf("bulle declares %d exchange entries, want courrier, agenda, opus, sablier, nuage and mycelium",
			len(bulle.TokenExchange))
	}
	for _, grant := range bulle.TokenExchange {
		prefix, ok := want[grant.Audience]
		if !ok || !slices.Equal(grant.Scopes, []string{prefix + ".read", prefix + ".write"}) {
			t.Errorf("%s allows %v, want exactly that app's read and write scopes", grant.Audience, grant.Scopes)
		}
	}
}

func loadProductionSeed(t *testing.T) File {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "seed.yaml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the production seed: %v", err)
	}
	for _, name := range secretEnvNames(string(body)) {
		t.Setenv(name, "placeholder")
	}
	file, err := Load(path)
	if err != nil {
		t.Fatalf("the production seed does not load: %v", err)
	}
	return file
}
