package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestUnknownSubcommandIsRefused pins the dispatch guard: an argument the
// binary does not answer prints the usage instead of starting a server, and
// no argument or a real subcommand passes through.
func TestUnknownSubcommandIsRefused(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h", "nosuchcommand", "user", "migrate-status"} {
		var stderr bytes.Buffer
		if knownSubcommand([]string{"/api", arg}, &stderr) {
			t.Errorf("knownSubcommand(%q) let it through to a server start", arg)
		}
		for _, name := range subcommands {
			if !strings.Contains(stderr.String(), name) {
				t.Errorf("usage for %q does not list %q:\n%s", arg, name, stderr.String())
			}
		}
	}
	passing := [][]string{{"/api"}, {"/api", ""}, {"/api", "migrate", "status"}, {"/api", "user-erase", "--email", "a@example.com"}}
	for _, name := range subcommands {
		passing = append(passing, []string{"/api", name})
	}
	for _, args := range passing {
		var stderr bytes.Buffer
		if !knownSubcommand(args, &stderr) || stderr.Len() != 0 {
			t.Errorf("knownSubcommand(%q) refused it: %q", args, stderr.String())
		}
	}
}
