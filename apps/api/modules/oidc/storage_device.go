package oidc

import (
	"context"
	"errors"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/devices"

	"github.com/zitadel/oidc/v3/pkg/op"
)

// op.DeviceAuthorizationStorage, RFC 8628. Implementing these two methods is
// also what turns the grant on: zitadel/oidc decides whether to advertise
// urn:ietf:params:oauth:grant-type:device_code, and whether the token endpoint
// will dispatch it at all, by type-asserting the storage for this interface.
// Until they existed, discovery advertised a device_authorization_endpoint
// that answered unauthorized_client to every caller.
//
// Everything either method decides lives in internal/devices. What is here is
// translation, because the library's vocabulary and registre's are not the
// same one: a domain service that spoke op.DeviceAuthorizationState would be
// modules/oidc wearing a different directory.

// StoreDeviceAuthorization records a flow a CLI just started.
//
// The misspelling in the sibling method's name is the library's; this one is
// spelled correctly, which is its own small trap when grepping.
func (s *Storage) StoreDeviceAuthorization(
	ctx context.Context, clientID, deviceCode, userCode string, expires time.Time, scopes []string,
) error {
	err := s.svc.Devices.Store(ctx, devices.StoreParams{
		ClientID:   clientID,
		DeviceCode: deviceCode,
		UserCode:   userCode,
		ExpiresAt:  expires,
		Scopes:     scopes,
	})
	if errors.Is(err, devices.ErrDuplicateUserCode) {
		return op.ErrDuplicateUserCode
	}
	return err
}

// GetDeviceAuthorizatonState answers one poll of the token endpoint.
//
// The name is misspelled — "Authorizaton" — in zitadel/oidc v3's interface,
// and the misspelling is load-bearing: correct it and the type assertion that
// enables the whole grant silently stops matching, discovery drops the grant,
// and the token endpoint answers unsupported_grant_type with nothing in any
// log to say a method was renamed.
//
// The three outcomes map onto the struct's two booleans and its Expires field,
// in the order the library checks them: denied first, then done, then expiry.
// A flow nobody has answered leaves both false and lets Expires decide between
// authorization_pending and expired_token.
//
// An error here becomes access_denied whatever it was — CheckDeviceAuthorizationState
// wraps every non-deadline error in oidc.ErrAccessDenied — so an unknown
// device code reads to the CLI as a refusal rather than as invalid_grant.
// That is the library's mapping and not something the storage can influence;
// it is recorded here so the next person to read the CLI's error message knows
// it is not lying about a refusal that happened.
func (s *Storage) GetDeviceAuthorizatonState(
	ctx context.Context, clientID, deviceCode string,
) (*op.DeviceAuthorizationState, error) {
	grant, err := s.svc.Devices.Poll(ctx, clientID, deviceCode)
	if err != nil {
		return nil, err
	}
	return &op.DeviceAuthorizationState{
		ClientID: grant.ClientID,
		Scopes:   grant.Scopes,
		Expires:  grant.Expires,
		Done:     grant.Approved,
		Denied:   grant.Denied,
		Subject:  grant.Subject,
		AMR:      grant.AMR,
		AuthTime: grant.AuthTime,
	}, nil
}
