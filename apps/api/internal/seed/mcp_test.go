package seed

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

func mcpDocument(courrier, bulle string) string {
	doc := exchangeDocument(bulle)
	return strings.Replace(doc, "    redirect_uris: [https://courrier.example/cb]\n",
		"    redirect_uris: [https://courrier.example/cb]\n"+courrier, 1)
}

func TestAnMCPBlockIsAppliedAndAReapplyClearsIt(t *testing.T) {
	t.Setenv("EXCHANGE_SECRET", "set")
	register, _ := register(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)
	with, err := Load(write(t, mcpDocument(
		"    mcp:\n      resource: https://courrier.example/mcp\n      scopes: [courrier.read, courrier.write]\n", "")))
	if err != nil {
		t.Fatalf("a valid block was refused: %v", err)
	}
	if err := Apply(ctx, register, with, logger); err != nil {
		t.Fatalf("apply: %v", err)
	}
	owner, err := register.ByResource(ctx, "https://courrier.example/mcp")
	if err != nil || owner.ClientID != "courrier-id" || !owner.MCPScopes.Contains("courrier.write") {
		t.Fatalf("ByResource = %+v (%v)", owner, err)
	}
	without, err := Load(write(t, mcpDocument("", "")))
	if err != nil {
		t.Fatalf("load without: %v", err)
	}
	if err := Apply(ctx, register, without, logger); err != nil {
		t.Fatalf("reapply: %v", err)
	}
	if _, err := register.ByResource(ctx, "https://courrier.example/mcp"); err == nil {
		t.Error("dropping the block left the resource declared")
	}
}

func TestAnMCPBlockThatCouldNotWorkFailsTheBoot(t *testing.T) {
	t.Setenv("EXCHANGE_SECRET", "set")
	block := func(resource, scopes string) string {
		return "    mcp:\n      resource: " + resource + "\n      scopes: " + scopes + "\n"
	}
	cases := map[string]string{
		"plain http":      mcpDocument(block("http://courrier.example/mcp", "[courrier.read]"), ""),
		"a query":         mcpDocument(block("https://courrier.example/mcp?x=1", "[courrier.read]"), ""),
		"a fragment":      mcpDocument(block("https://courrier.example/mcp#x", "[courrier.read]"), ""),
		"no scopes":       mcpDocument(block("https://courrier.example/mcp", "[]"), ""),
		"another prefix":  mcpDocument(block("https://courrier.example/mcp", "[agenda.read]"), ""),
		"an unknown verb": mcpDocument(block("https://courrier.example/mcp", "[courrier.admin]"), ""),
		"one URI, two apps": mcpDocument(block("https://courrier.example/mcp", "[courrier.read]"),
			"    mcp:\n      resource: https://courrier.example/mcp\n      scopes: [bulle.read]"),
	}
	for name, document := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(write(t, document)); err == nil {
				t.Errorf("a seed with %s loaded", name)
			}
		})
	}
}

func TestTheProductionSeedDeclaresTheSuiteMCPServers(t *testing.T) {
	file := loadProductionSeed(t)
	want := map[string]string{
		"Courrier": "https://courrier.facile.studio/mcp",
		"Agenda":   "https://agenda.facile.studio/mcp",
		"Nuage":    "https://nuage.facile.studio/mcp",
	}
	for _, application := range file.Applications {
		resource, ok := want[application.Name]
		if !ok {
			continue
		}
		if application.MCP == nil || application.MCP.Resource != resource {
			t.Errorf("%s declares %+v, want %s", application.Name, application.MCP, resource)
		}
		delete(want, application.Name)
	}
	if len(want) > 0 {
		t.Errorf("the production seed is missing %v", want)
	}
}
