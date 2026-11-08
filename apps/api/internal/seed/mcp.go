package seed

import (
	"fmt"
	"net/url"
)

// MCP declares the MCP server an application runs: its canonical URI, which
// becomes the only audience of a token asked for it (RFC 8707), and the
// scopes such a token may carry.
//
// The URI is what a person types into an MCP client, so it is compared byte
// for byte and must be the exact string the server publishes as `resource` in
// its protected resource metadata. The scopes follow the exchange vocabulary,
// <app>.read and <app>.write, so a token exchanged by Bulle and a token a
// third-party client earned through consent read the same at the server.
type MCP struct {
	Resource string   `yaml:"resource"`
	Scopes   []string `yaml:"scopes"`
}

// validateMCP rejects every mcp block that could not be applied, across the
// file because two applications claiming one URI would fail the unique index
// at boot and take the provider down with it.
func (f File) validateMCP() error {
	owners := map[string]string{}
	for index, application := range f.Applications {
		if application.MCP == nil {
			continue
		}
		where := fmt.Sprintf("applications[%d] (%s): mcp", index, application.ClientID)
		if err := application.MCP.validate(where, exchangePrefix(application.Name)); err != nil {
			return err
		}
		if owner, taken := owners[application.MCP.Resource]; taken {
			return fmt.Errorf("%s: resource %q is already declared by %s", where, application.MCP.Resource, owner)
		}
		owners[application.MCP.Resource] = application.ClientID
	}
	return nil
}

// validate rejects one mcp block.
func (m MCP) validate(where, prefix string) error {
	parsed, err := url.Parse(m.Resource)
	switch {
	case m.Resource == "":
		return fmt.Errorf("%s: resource is required", where)
	case err != nil || parsed.Scheme != "https" || parsed.Host == "":
		return fmt.Errorf("%s: resource %q must be an absolute https URL", where, m.Resource)
	case parsed.Fragment != "" || parsed.RawQuery != "" || parsed.User != nil:
		return fmt.Errorf("%s: resource %q must carry no query, fragment or credentials (RFC 8707 §2)", where, m.Resource)
	case len(m.Scopes) == 0:
		return fmt.Errorf("%s: at least one scope is required", where)
	case prefix == "":
		return fmt.Errorf("%s: the application has no name to derive a scope prefix from", where)
	}
	for _, scope := range m.Scopes {
		if !exchangeScopeOK(prefix, scope) {
			return fmt.Errorf("%s: scope %q must be %s.read or %s.write", where, scope, prefix, prefix)
		}
	}
	return nil
}

// mcpColumns is the stored form of the block: the resource and its scopes, or
// two empty values for an application that runs no MCP server.
func (a Application) mcpColumns() (string, []string) {
	if a.MCP == nil {
		return "", nil
	}
	return a.MCP.Resource, a.MCP.Scopes
}
