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
// It is emitted only when the connections scope was granted and the client
// opted into at least one provider through login_providers, and then it lists
// only those providers, as an empty object when none is linked. A client that
// opted into nothing gets no claim, silently, whatever scope it asked for.
func (s *Storage) appendConnections(ctx context.Context, userinfo *zoidc.UserInfo, userID, clientID string) error {
	if s.svc.Connections == nil {
		return nil
	}
	connections, err := s.svc.Connections.Connections(ctx, userID, clientID)
	if err != nil {
		return fmt.Errorf("oidc: read the connections: %w", err)
	}
	if connections == nil {
		return nil
	}
	userinfo.AppendClaims(scopeConnections, connections)
	return nil
}
