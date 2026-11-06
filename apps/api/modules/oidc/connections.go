package oidc

import (
	"context"
	"fmt"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
)

// appendConnections adds the connections claim: the provider-to-subject map
// of the external accounts userID has linked, such as
// {"discord": "123456789012345678"}.
//
// It is emitted only when the connections scope was granted, and then always,
// as an empty object when nothing is linked, for the same reason roles is: an
// absent claim and an empty one are different answers to a relying party that
// reads the claim to decide whether the provider supports it at all.
func (s *Storage) appendConnections(ctx context.Context, userinfo *zoidc.UserInfo, userID string) error {
	connections := map[string]string{}
	if s.svc.Connections != nil {
		read, err := s.svc.Connections.Connections(ctx, userID)
		if err != nil {
			return fmt.Errorf("oidc: read the connections: %w", err)
		}
		connections = read
	}
	userinfo.AppendClaims(scopeConnections, connections)
	return nil
}
