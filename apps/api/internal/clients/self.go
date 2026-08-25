package clients

import (
	"context"
	"fmt"

	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
)

// SelfClientID is the id registre registers itself under.
//
// SPEC §5 says registre cannot be its own relying party, and for sessions that
// is still true: internal/accounts exists precisely because registre has
// nowhere to get a session from. This is the one narrow exception, and it does
// not contradict that. registre is not consuming its own tokens — it discards
// the authorization code this client produces. It is using its own front door
// so that a human who arrives at the device verification page with no session
// can get one, through the same /authorize → /login → complete path every
// other app uses and that is already tested.
const SelfClientID = "registre"

// EnsureSelf registers registre's own client, or brings it up to date.
//
// It is code rather than a seed entry on purpose. The seed is optional —
// REGISTRE_SEED_FILE is unset on a development instance and in every test — and
// a door that only exists when a YAML file happens to be mounted is a door that
// is missing exactly when somebody is debugging why they cannot get in. It also
// needs no secret to be provisioned, so it adds no way for a boot to fail.
//
// The row is public, because the party using it is a browser: there is nowhere
// to keep a secret in a page. It asks for openid and nothing else, because the
// only thing this flow is for is the account cookie login writes on the way
// through, and every extra scope would be a claim nobody reads. Managed stays
// false so the seed's "declared clients you have dropped" report does not name
// a row the seed never owned.
//
// It runs after the seed, so a file that declares a client called registre
// cannot quietly take the hostname's own login page with it.
func (s *Service) EnsureSelf(ctx context.Context, redirectURI string) error {
	subject, err := ids.New()
	if err != nil {
		return fmt.Errorf("clients: mint a subject for %s: %w", SelfClientID, err)
	}
	_, err = s.Upsert(ctx, schemas.Client{
		ClientID: SelfClientID, Subject: subject, Name: "Registre",
		Kind: schemas.KindApplication, Public: true,
		GrantTypes:   schemas.StringList{string(zoidc.GrantTypeCode)},
		Scopes:       schemas.StringList{string(zoidc.ScopeOpenID)},
		RedirectURIs: schemas.StringList{redirectURI},
	})
	return err
}
