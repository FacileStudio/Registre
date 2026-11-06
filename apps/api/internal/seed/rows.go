package seed

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/ids"
	"github.com/FacileStudio/Registre/apps/api/schemas"

	zoidc "github.com/zitadel/oidc/v3/pkg/oidc"
)

// grantAliases is what an application may declare, and what each name means on
// the wire.
//
// The aliases exist because the device grant's real name is
// urn:ietf:params:oauth:grant-type:device_code, and a URN in a YAML file is a
// string nobody proofreads — a typo in it registers a client that is refused at
// the token endpoint with unauthorized_client and no clue which of the two
// spellings was wrong.
//
// client_credentials is deliberately absent. A client that authenticates as
// itself rather than on behalf of a human is a service account, and this file
// already has a section for those; letting an application declare the grant
// would make the two kinds mean the same thing while still being counted
// separately everywhere else.
var grantAliases = map[string]string{
	"authorization_code": string(zoidc.GrantTypeCode),
	"refresh_token":      string(zoidc.GrantTypeRefreshToken),
	"device_code":        string(zoidc.GrantTypeDeviceCode),
}

// defaultGrants is what an application that declares none gets: the browser
// login and its renewal, which is what every suite app registered before grant
// types could be declared at all.
var defaultGrants = []string{"authorization_code", "refresh_token"}

// redirectingGrant is the one grant that sends a browser back somewhere, and
// therefore the one that makes redirect_uris mandatory. A device flow has no
// redirect to validate — that is the whole reason it exists — so requiring one
// of a CLI would be requiring a URL nobody would ever visit.
const redirectingGrant = "authorization_code"

// validate rejects a document that could not be applied, before anything is
// written and while the error can still name the line the reader has open.
func (f File) validate() error {
	seen := map[string]bool{}
	for index, account := range f.ServiceAccounts {
		where := fmt.Sprintf("service_accounts[%d]", index)
		switch {
		case account.ClientID == "":
			return fmt.Errorf("%s: client_id is required", where)
		case seen[account.ClientID]:
			return fmt.Errorf("%s: client_id %q is declared twice", where, account.ClientID)
		case account.SecretEnv == "":
			return fmt.Errorf("%s (%s): secret_env is required", where, account.ClientID)
		}
		seen[account.ClientID] = true
		if _, err := account.lifetime(); err != nil {
			return fmt.Errorf("%s (%s): %w", where, account.ClientID, err)
		}
		if os.Getenv(account.SecretEnv) == "" {
			return fmt.Errorf(
				"%s (%s): secret_env %s is unset — a declared account with no credential cannot authenticate",
				where, account.ClientID, account.SecretEnv)
		}
	}
	for index, application := range f.Applications {
		where := fmt.Sprintf("applications[%d]", index)
		if seen[application.ClientID] {
			return fmt.Errorf("%s: client_id %q is declared twice", where, application.ClientID)
		}
		if err := application.validate(where); err != nil {
			return err
		}
		seen[application.ClientID] = true
	}
	return f.validateExchange()
}

// validate rejects one application declaration.
//
// The secret rule has two halves and both matter. A confidential app whose
// secret_env is unset fails the boot, because a client nobody can authenticate
// as is a login that will fail later with a message about credentials. A
// public app that names a secret_env fails too, because it would read as if the
// secret were being used — and the whole point of declaring a client public is
// that nothing checks one.
func (a Application) validate(where string) error {
	switch {
	case a.ClientID == "":
		return fmt.Errorf("%s: client_id is required", where)
	case a.Public && a.SecretEnv != "":
		return fmt.Errorf(
			"%s (%s): a public client holds no secret, so secret_env must not be set — "+
				"drop it, or drop public",
			where, a.ClientID)
	case !a.Public && a.SecretEnv == "":
		return fmt.Errorf("%s (%s): secret_env is required", where, a.ClientID)
	}
	grants, err := a.grants()
	if err != nil {
		return fmt.Errorf("%s (%s): %w", where, a.ClientID, err)
	}
	if contains(grants, grantAliases[redirectingGrant]) && len(a.RedirectURIs) == 0 {
		return fmt.Errorf("%s (%s): at least one redirect URI is required — "+
			"an application that cannot state where its login returns to cannot be sent anywhere safely",
			where, a.ClientID)
	}
	if !a.Public && os.Getenv(a.SecretEnv) == "" {
		return fmt.Errorf(
			"%s (%s): secret_env %s is unset — a declared app with no credential cannot authenticate",
			where, a.ClientID, a.SecretEnv)
	}
	return nil
}

// grants resolves the declared aliases into the grant type names the token
// endpoint compares against.
func (a Application) grants() ([]string, error) {
	declared := a.GrantTypes
	if len(declared) == 0 {
		declared = defaultGrants
	}
	resolved := make([]string, 0, len(declared))
	for _, alias := range declared {
		name, ok := grantAliases[alias]
		if !ok {
			return nil, fmt.Errorf("grant_types: %q is not one of authorization_code, refresh_token, device_code", alias)
		}
		resolved = append(resolved, name)
	}
	return a.withExchangeGrant(resolved), nil
}

// contains reports whether list holds value.
func contains(list []string, value string) bool {
	for _, candidate := range list {
		if candidate == value {
			return true
		}
	}
	return false
}

// lifetime parses the declared access token lifetime.
func (a ServiceAccount) lifetime() (time.Duration, error) {
	if a.AccessTokenTTL == "" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(a.AccessTokenTTL)
	if err != nil {
		return 0, fmt.Errorf("access_token_ttl %q is not a duration: %w", a.AccessTokenTTL, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("access_token_ttl %q is not positive", a.AccessTokenTTL)
	}
	return parsed, nil
}

// row turns an application declaration into the register row it becomes,
// under the same hashing ceiling as every other credential path.
func (a Application) row(ctx context.Context, register *clients.Service) (schemas.Client, error) {
	hash, err := a.hash(ctx, register)
	if err != nil {
		return schemas.Client{}, err
	}
	grants, err := a.grants()
	if err != nil {
		return schemas.Client{}, fmt.Errorf("seed: %s: %w", a.ClientID, err)
	}
	if a.DeletionURI != "" && a.DeletionURI == a.DirectorySyncURI {
		return schemas.Client{}, fmt.Errorf("seed: %s: deletion_uri must not be the directory_sync_uri, "+
			"a sync handler accepts any POST and erases nothing", a.ClientID)
	}
	subject, err := ids.New()
	if err != nil {
		return schemas.Client{}, fmt.Errorf("seed: mint a subject for %s: %w", a.ClientID, err)
	}
	scopes := a.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", "email"}
	}
	return schemas.Client{
		ClientID: a.ClientID, Subject: subject, Name: a.Name,
		Kind: schemas.KindApplication, SecretHash: hash, Public: a.Public,
		GrantTypes:           schemas.StringList(grants),
		Scopes:               schemas.StringList(scopes),
		RedirectURIs:         schemas.StringList(a.RedirectURIs),
		BackchannelLogoutURI: a.BackchannelLogoutURI,
		DirectorySyncURI:     a.DirectorySyncURI,
		DeletionURI:          a.DeletionURI,
		VisibleRoles:         schemas.StringList(a.VisibleRoles),
		TokenExchange:        a.exchangeGrants(),
		Managed:              true,
	}, nil
}

// hash seals the declared secret, or returns none for a public client.
//
// The empty-secret check repeats validate's, which is not an oversight: that
// one exists to fail the boot with a message naming the variable to set, and
// this one exists because hashing an empty string produces a perfectly valid
// hash that an empty secret then matches. A public client skips both, and the
// register refuses it a password path of its own — Public is what the token
// endpoint reads, not the absence of a hash.
func (a Application) hash(ctx context.Context, register *clients.Service) (string, error) {
	if a.Public {
		return "", nil
	}
	secret := os.Getenv(a.SecretEnv)
	if secret == "" {
		return "", fmt.Errorf("seed: %s has no secret in %s", a.ClientID, a.SecretEnv)
	}
	hash, err := register.HashSecret(ctx, secret)
	if err != nil {
		return "", fmt.Errorf("seed: hash the secret for %s: %w", a.ClientID, err)
	}
	return hash, nil
}

// row turns a declaration into the register row it becomes.
//
// The empty-secret check repeats validate's for the reason given on
// Application.hash above: only one of the two is load-bearing for security,
// and it is this one.
//
// Hashing goes through the register rather than through porte directly, so the
// seed sits under the same argon2 ceiling every other caller does.
func (a ServiceAccount) row(ctx context.Context, register *clients.Service) (schemas.Client, error) {
	secret := os.Getenv(a.SecretEnv)
	if secret == "" {
		return schemas.Client{}, fmt.Errorf("seed: %s has no secret in %s", a.ClientID, a.SecretEnv)
	}
	hash, err := register.HashSecret(ctx, secret)
	if err != nil {
		return schemas.Client{}, fmt.Errorf("seed: hash the secret for %s: %w", a.ClientID, err)
	}
	subject, err := ids.New()
	if err != nil {
		return schemas.Client{}, fmt.Errorf("seed: mint a subject for %s: %w", a.ClientID, err)
	}
	lifetime, err := a.lifetime()
	if err != nil {
		return schemas.Client{}, err
	}
	return schemas.Client{
		ClientID: a.ClientID, Subject: subject, Name: a.Name,
		Kind: schemas.KindServiceAccount, SecretHash: hash,
		GrantTypes:     schemas.StringList{string(zoidc.GrantTypeClientCredentials)},
		Scopes:         schemas.StringList(a.Scopes),
		Audiences:      schemas.StringList(a.Audiences),
		AccessTokenTTL: int64(lifetime / time.Second),
		Managed:        true,
	}, nil
}
