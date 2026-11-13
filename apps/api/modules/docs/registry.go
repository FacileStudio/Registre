package docs

import (
	"net/http"

	"github.com/FacileStudio/tronc/apiref"
)

type (
	Registry = apiref.Registry
	Module   = apiref.Module
	Route    = apiref.Route
	Field    = apiref.Field
	Error    = apiref.Error
)

type DiscoveryResponse struct {
	Issuer                           string   `json:"issuer"`
	AuthorizationEndpoint            string   `json:"authorization_endpoint"`
	TokenEndpoint                    string   `json:"token_endpoint"`
	UserinfoEndpoint                 string   `json:"userinfo_endpoint"`
	JwksURI                          string   `json:"jwks_uri"`
	ResponseTypesSupported           []string `json:"response_types_supported"`
	SubjectTypesSupported            []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
}

type TokenRequest struct {
	GrantType    string `json:"grant_type" validate:"required"`
	ClientID     string `json:"client_id" validate:"required"`
	ClientSecret string `json:"client_secret,omitempty"`
	Code         string `json:"code,omitempty"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	DeviceCode   string `json:"device_code,omitempty"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in"`
	IDToken      string `json:"id_token,omitempty"`
}

type IntrospectRequest struct {
	Token string `json:"token" validate:"required"`
}

type IntrospectResponse struct {
	Active   bool     `json:"active"`
	Scope    string   `json:"scope,omitempty"`
	ClientID string   `json:"client_id,omitempty"`
	Sub      string   `json:"sub,omitempty"`
	Exp      int64    `json:"exp,omitempty"`
	Audience []string `json:"aud,omitempty"`
}

type RevokeRequest struct {
	Token string `json:"token" validate:"required"`
}

type UserInfoResponse struct {
	Sub               string   `json:"sub"`
	Email             string   `json:"email,omitempty"`
	EmailVerified     bool     `json:"email_verified,omitempty"`
	PreferredUsername string   `json:"preferred_username,omitempty"`
	Roles             []string `json:"roles,omitempty"`
}

type DeviceAuthRequest struct {
	ClientID string `json:"client_id" validate:"required"`
	Scope    string `json:"scope,omitempty"`
}

type DeviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// SessionLoginRequest names the auth request an existing session finishes.
type SessionLoginRequest struct {
	AuthRequest string `json:"auth_request" validate:"required"`
}

type LoginResponse struct {
	RedirectURI  string `json:"redirect_uri,omitempty"`
	SecondFactor bool   `json:"second_factor,omitempty"`
	ChallengeID  string `json:"challenge_id,omitempty"`
}

type SecondFactorRequest struct {
	ChallengeID string `json:"challenge_id" validate:"required"`
	Code        string `json:"code" validate:"required"`
}

type BrowserConfigResponse struct {
	JournalURL string `json:"journal_url,omitempty"`
	JournalKey string `json:"journal_key,omitempty"`
}

type UserProfileResponse struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"is_admin"`
	HasTOTP bool   `json:"has_totp"`
	Picture string `json:"picture"`
}

type SpacesMember struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Role    string `json:"role"`
}

type SpacesSpace struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Members      []SpacesMember `json:"members"`
	Entitlements map[string]any `json:"entitlements,omitempty"`
}

type SpacesPerson struct {
	Sub          string         `json:"sub"`
	Entitlements map[string]any `json:"entitlements,omitempty"`
}

type SpacesSnapshot struct {
	Version              int64          `json:"version"`
	EntitlementsRevision int64          `json:"entitlements_revision,omitempty"`
	Spaces               []SpacesSpace  `json:"spaces"`
	People               []SpacesPerson `json:"people,omitempty"`
}

type EntitlementsPush struct {
	Revision int64 `json:"revision"`
	Spaces   []struct {
		ID           string         `json:"id"`
		Entitlements map[string]any `json:"entitlements"`
	} `json:"spaces"`
	People []struct {
		Sub          string         `json:"sub,omitempty"`
		Email        string         `json:"email,omitempty"`
		Entitlements map[string]any `json:"entitlements"`
	} `json:"people"`
}

type SpacePlan struct {
	Enabled   bool             `json:"enabled"`
	Available bool             `json:"available"`
	Owner     bool             `json:"owner"`
	Plan      map[string]any   `json:"plan,omitempty"`
	Catalog   []map[string]any `json:"catalog,omitempty"`
}

type BillingCheckout struct {
	LookupKey string `json:"lookup_key"`
}

type SpacesImport struct {
	Spaces []struct {
		Name    string `json:"name"`
		Members []struct {
			Sub   string `json:"sub,omitempty"`
			Email string `json:"email,omitempty"`
			Role  string `json:"role"`
		} `json:"members"`
	} `json:"spaces"`
}

type SpaceSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	MemberCount int    `json:"member_count"`
}

type SpaceMember struct {
	UserID  string `json:"user_id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Role    string `json:"role"`
}

type SpaceDetail struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Role    string        `json:"role"`
	Members []SpaceMember `json:"members"`
}

type SpaceName struct {
	Name string `json:"name"`
}

type SpaceInvite struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type SpaceRole struct {
	Role string `json:"role"`
}

type AvatarResponse struct {
	Picture string `json:"picture"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required"`
}

type TOTPBeginResponse struct {
	Secret        string   `json:"secret"`
	QRCode        string   `json:"qr_code"`
	RecoveryCodes []string `json:"recovery_codes"`
}

type TOTPConfirmRequest struct {
	Code string `json:"code" validate:"required"`
}

type TOTPDisableRequest struct {
	Password string `json:"password" validate:"required"`
}

type RecoveryCodesResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

type PendingDeviceResponse struct {
	ClientName string   `json:"client_name"`
	ClientURI  string   `json:"client_uri,omitempty"`
	Scopes     []string `json:"scopes"`
	UserCode   string   `json:"user_code"`
}

type DeviceDecisionRequest struct {
	UserCode string `json:"user_code" validate:"required"`
}

type RecoverRequest struct {
	Email string `json:"email" validate:"required"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required"`
}

type ClientResponse struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	RedirectURIs []string `json:"redirect_uris"`
	Scopes       []string `json:"scopes"`
}

type CreateClientRequest struct {
	ID           string   `json:"id" validate:"required"`
	Name         string   `json:"name" validate:"required"`
	Description  string   `json:"description,omitempty"`
	RedirectURIs []string `json:"redirect_uris"`
	Scopes       []string `json:"scopes"`
}

type ClientCreatedResponse struct {
	Client       ClientResponse `json:"client"`
	ClientSecret string         `json:"client_secret"`
}

type RotateSecretResponse struct {
	ClientSecret string `json:"client_secret"`
}

type UserResponse struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	IsAdmin  bool   `json:"is_admin"`
	Disabled bool   `json:"disabled"`
}

// AccessPolicyRequest switches who may sign in to an application.
type AccessPolicyRequest struct {
	AccessPolicy string `json:"access_policy" validate:"required" enum:"everyone,assigned"`
}

// GrantRequest sets the roles a user holds in an application.
type GrantRequest struct {
	Roles []string `json:"roles"`
}

// GrantResponse is one person's access to an application.
type GrantResponse struct {
	ClientID string   `json:"client_id"`
	UserID   string   `json:"user_id"`
	Roles    []string `json:"roles"`
}

// ClientAccessResponse is an application's policy and its grants.
type ClientAccessResponse struct {
	ClientID     string          `json:"client_id"`
	AccessPolicy string          `json:"access_policy"`
	VisibleRoles []string        `json:"visible_roles"`
	Self         bool            `json:"self"`
	Grants       []GrantResponse `json:"grants"`
}

// OKResponse acknowledges a change that has nothing else to report.
type OKResponse struct {
	OK bool `json:"ok"`
}

// AdminEventResponse is one change an operator made.
type AdminEventResponse struct {
	ID         int64  `json:"id"`
	At         string `json:"at"`
	ActorID    string `json:"actor_id"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
}

type CreateUserRequest struct {
	Email    string `json:"email" validate:"required"`
	Name     string `json:"name" validate:"required"`
	Password string `json:"password" validate:"required"`
	IsAdmin  bool   `json:"is_admin"`
}

type InviteResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	ExpiresAt string `json:"expires_at"`
	InviteURL string `json:"invite_url"`
}

type CreateInviteRequest struct {
	Email   string `json:"email" validate:"required"`
	IsAdmin bool   `json:"is_admin"`
}

type KeyStatusResponse struct {
	ActiveKeyID   string   `json:"active_key_id"`
	PublishedKeys []string `json:"published_keys"`
	NextRotation  string   `json:"next_rotation"`
}

type RedeemInviteRequest struct {
	Token    string `json:"token" validate:"required"`
	Name     string `json:"name" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// Reference is Registre's API registry, served at /docs.
func Reference() apiref.Config {
	cfg := apiref.Config{
		Title:       "Registre API",
		Description: "Central OpenID Provider and Identity Management for the Facile Suite.",
		Servers:     []string{},
		Registry: Registry{
			Modules: []Module{
				{
					Name:        "oidc",
					Description: "Standard OpenID Connect and OAuth 2.0 protocol endpoints",
					Routes: []Route{
						{
							Method:       "GET",
							Path:         "/.well-known/openid-configuration",
							Summary:      "OpenID Provider discovery document",
							ResponseBody: DiscoveryResponse{},
						},
						{
							Method:       "GET",
							Path:         "/.well-known/oauth-authorization-server",
							Summary:      "OAuth 2.0 authorization server metadata (RFC 8414), the discovery document",
							ResponseBody: DiscoveryResponse{},
						},
						{
							Method:       "POST",
							Path:         "/register",
							Summary:      "Dynamic client registration (RFC 7591) for public MCP clients; no secret is issued",
							RequestBody:  map[string]any{"redirect_uris": []string{}, "client_name": ""},
							Status:       http.StatusCreated,
							ResponseBody: map[string]any{"client_id": "", "token_endpoint_auth_method": "none"},
						},
						{
							Method:       "GET",
							Path:         "/oauth/v2/keys",
							Summary:      "JSON Web Key Set (JWKS)",
							ResponseBody: map[string]any{"keys": []any{}},
						},
						{
							Method:       "POST",
							Path:         "/oauth/v2/token",
							Summary:      "OAuth 2.0 token issuance",
							RequestBody:  TokenRequest{},
							ResponseBody: TokenResponse{},
						},
						{
							Method:       "POST",
							Path:         "/oauth/v2/introspect",
							Summary:      "OAuth 2.0 token introspection (RFC 7662)",
							Auth:         "bearer",
							RequestBody:  IntrospectRequest{},
							ResponseBody: IntrospectResponse{},
						},
						{
							Method:       "POST",
							Path:         "/oauth/v2/revoke",
							Summary:      "OAuth 2.0 token revocation (RFC 7009)",
							Auth:         "bearer",
							RequestBody:  RevokeRequest{},
							Status:       http.StatusOK,
							ResponseBody: "OK",
						},
						{
							Method:       "GET",
							Path:         "/oauth/v2/userinfo",
							Summary:      "OpenID Connect UserInfo endpoint",
							Auth:         "bearer",
							ResponseBody: UserInfoResponse{},
						},
						{
							Method:       "POST",
							Path:         "/oauth/v2/userinfo",
							Summary:      "OpenID Connect UserInfo endpoint (POST form/bearer)",
							Auth:         "bearer",
							ResponseBody: UserInfoResponse{},
						},
						{
							Method:       "GET",
							Path:         "/oauth/v2/authorize",
							Summary:      "OAuth 2.0 authorization endpoint",
							ResponseBody: "HTML or redirect",
						},
						{
							Method:       "POST",
							Path:         "/oauth/v2/authorize/callback",
							Summary:      "OAuth 2.0 authorization callback",
							ResponseBody: "HTML or redirect",
						},
						{
							Method:       "GET",
							Path:         "/oauth/v2/sessions/end",
							Summary:      "OpenID Connect RP-Initiated Logout",
							ResponseBody: "HTML or redirect",
						},
						{
							Method:       "POST",
							Path:         "/oauth/v2/sessions/end",
							Summary:      "OpenID Connect RP-Initiated Logout (POST)",
							ResponseBody: "HTML or redirect",
						},
						{
							Method:       "POST",
							Path:         "/oauth/v2/device",
							Summary:      "OAuth 2.0 Device Authorization (RFC 8628)",
							RequestBody:  DeviceAuthRequest{},
							ResponseBody: DeviceAuthResponse{},
						},
					},
				},
				{
					Name:        "login",
					Description: "Human login and session initialization",
					Routes: []Route{
						{
							Method:       "POST",
							Path:         "/api/login",
							Summary:      "Authenticate with email and password",
							RequestBody:  LoginRequest{},
							ResponseBody: LoginResponse{},
						},
						{
							Method:       "POST",
							Path:         "/api/login/totp",
							Summary:      "Verify second factor challenge",
							RequestBody:  SecondFactorRequest{},
							ResponseBody: LoginResponse{},
						},
						{
							Method:       "GET",
							Path:         "/api/login/session",
							Summary:      "Report whether the browser holds an account session",
							ResponseBody: LoginResponse{},
						},
						{
							Method:       "POST",
							Path:         "/api/login/session",
							Summary:      "Finish an auth request with the existing session (needs X-Facile-CSRF)",
							RequestBody:  SessionLoginRequest{},
							ResponseBody: LoginResponse{},
						},
						{
							Method:       "GET",
							Path:         "/api/config",
							Summary:      "Public browser client configuration",
							ResponseBody: BrowserConfigResponse{},
						},
					},
				},
				{
					Name:        "account",
					Description: "Self-service account management and security settings",
					Routes: []Route{
						{
							Method:       "GET",
							Path:         "/api/account/",
							Summary:      "Get authenticated user profile",
							Auth:         "cookie",
							ResponseBody: UserProfileResponse{},
						},
						{
							Method:      "POST",
							Path:        "/api/account/password",
							Summary:     "Change account password",
							Auth:        "cookie",
							RequestBody: ChangePasswordRequest{},
							Status:      http.StatusNoContent,
						},
						{
							Method:       "PUT",
							Path:         "/api/account/avatar",
							Summary:      "Replace the profile picture",
							Description:  "The body is the raw image. It is cropped to a square and re-encoded as PNG.",
							Auth:         "cookie",
							RequestBody:  "image/png, image/jpeg or image/gif, at most 4 MiB, 16 to 2048 px a side",
							ResponseBody: AvatarResponse{},
							Errors: []Error{
								{Status: http.StatusBadRequest, Code: "invalid_picture", Description: "Not a usable image"},
								{Status: http.StatusRequestEntityTooLarge, Code: "too_large", Description: "Over 4 MiB"},
							},
						},
						{
							Method:       "DELETE",
							Path:         "/api/account/avatar",
							Summary:      "Remove the profile picture",
							Auth:         "cookie",
							ResponseBody: AvatarResponse{},
						},
						{
							Method:       "POST",
							Path:         "/api/account/totp/begin",
							Summary:      "Initiate TOTP setup",
							Auth:         "cookie",
							ResponseBody: TOTPBeginResponse{},
						},
						{
							Method:      "POST",
							Path:        "/api/account/totp/confirm",
							Summary:     "Confirm and activate TOTP setup",
							Auth:        "cookie",
							RequestBody: TOTPConfirmRequest{},
							Status:      http.StatusNoContent,
						},
						{
							Method:      "POST",
							Path:        "/api/account/totp/disable",
							Summary:     "Disable TOTP on account",
							Auth:        "cookie",
							RequestBody: TOTPDisableRequest{},
							Status:      http.StatusNoContent,
						},
						{
							Method:       "POST",
							Path:         "/api/account/totp/recovery-codes",
							Summary:      "Regenerate backup recovery codes",
							Auth:         "cookie",
							ResponseBody: RecoveryCodesResponse{},
						},
						{
							Method:       "POST",
							Path:         "/api/account/logout",
							Summary:      "End the account session and sign out of every app it reached",
							Auth:         "cookie",
							ResponseBody: map[string]any{"logged_out": true},
						},
					},
				},
				{
					Name:        "directory",
					Description: "Shared spaces for the suite's apps, authenticated with the app's own client id and secret (HTTP Basic)",
					Routes: []Route{
						{Method: "GET", Path: "/api/directory/snapshot", Summary: "Every shared space with its members and roles", Auth: "basic", ResponseBody: SpacesSnapshot{}},
						{Method: "POST", Path: "/api/directory/import", Summary: "Merge the calling app's existing spaces by name, once per app", Auth: "basic", RequestBody: SpacesImport{}, ResponseBody: SpacesSnapshot{}},
						{Method: "PUT", Path: "/api/directory/entitlements", Summary: "Replace every plan entitlement (ENTITLEMENTS_WRITER only); 204, 409 on a stale revision", Auth: "basic", RequestBody: EntitlementsPush{}, Status: 204},
					},
				},
				{
					Name:        "spaces",
					Description: "The spaces page: create teams, invite mates and set roles for every app at once",
					Routes: []Route{
						{Method: "GET", Path: "/api/spaces/", Summary: "Spaces I am in", Auth: "cookie", ResponseBody: []SpaceSummary{}},
						{Method: "POST", Path: "/api/spaces/", Summary: "Create a space I own", Auth: "cookie", RequestBody: SpaceName{}, ResponseBody: SpaceSummary{}},
						{Method: "GET", Path: "/api/spaces/{spaceID}", Summary: "One space with its members", Auth: "cookie", ResponseBody: SpaceDetail{}},
						{Method: "POST", Path: "/api/spaces/{spaceID}/rename", Summary: "Rename a space (admin or owner)", Auth: "cookie", RequestBody: SpaceName{}, ResponseBody: SpaceName{}},
						{Method: "POST", Path: "/api/spaces/{spaceID}/members", Summary: "Invite a person by email (admin or owner); 202 whether or not they have an account, nobody joins until they accept", Auth: "cookie", RequestBody: SpaceInvite{}, ResponseBody: map[string]string{"status": "invited"}, Status: http.StatusAccepted},
						{Method: "POST", Path: "/api/spaces/{spaceID}/members/{userID}/role", Summary: "Change a member's role", Auth: "cookie", RequestBody: SpaceRole{}, ResponseBody: map[string]any{"member": SpaceRole{}}},
						{Method: "DELETE", Path: "/api/spaces/{spaceID}/members/{userID}", Summary: "Remove a member", Auth: "cookie", ResponseBody: map[string]bool{"ok": true}},
						{Method: "DELETE", Path: "/api/spaces/{spaceID}", Summary: "Delete a space in every app (owner only)", Auth: "cookie", ResponseBody: map[string]bool{"ok": true}},
						{Method: "POST", Path: "/api/spaces/{spaceID}/leave", Summary: "Leave a space", Auth: "cookie", RequestBody: map[string]any{}, ResponseBody: map[string]bool{"ok": true}},
						{Method: "GET", Path: "/api/spaces/{spaceID}/billing", Summary: "The space's plan, and the catalog for owners", Auth: "cookie", ResponseBody: SpacePlan{}},
						{Method: "POST", Path: "/api/spaces/{spaceID}/billing/checkout", Summary: "Start a Stripe checkout for a published price (owner only)", Auth: "cookie", RequestBody: BillingCheckout{}, ResponseBody: map[string]string{"url": ""}},
						{Method: "POST", Path: "/api/spaces/{spaceID}/billing/portal", Summary: "Open the Stripe billing portal (owner only)", Auth: "cookie", RequestBody: map[string]any{}, ResponseBody: map[string]string{"url": ""}},
					},
				},
				{
					Name:        "avatars",
					Description: "Public profile pictures, the URLs the picture claim carries",
					Routes: []Route{
						{
							Method:       "GET",
							Path:         "/avatars/{key}",
							Summary:      "Fetch a profile picture",
							Description:  "Public. The key is the user subject; the response revalidates (no-cache, ETag).",
							PathParams:   []Field{{Name: "key", Type: "string", Description: "User subject"}},
							ResponseBody: "image/png",
						},
					},
				},
				{
					Name:        "device",
					Description: "CLI Device Authorization verification",
					Routes: []Route{
						{
							Method:       "GET",
							Path:         "/api/device/authenticate",
							Summary:      "Initiate device approval sign-in round trip",
							ResponseBody: "Redirect to login",
						},
						{
							Method:       "GET",
							Path:         "/api/device/",
							Summary:      "Get pending device authorization details",
							Auth:         "cookie",
							QueryParams:  []Field{{Name: "user_code", Type: "string", Description: "User code"}},
							ResponseBody: PendingDeviceResponse{},
						},
						{
							Method:      "POST",
							Path:        "/api/device/approve",
							Summary:     "Approve device authorization code",
							Auth:        "cookie",
							RequestBody: DeviceDecisionRequest{},
							Status:      http.StatusNoContent,
						},
						{
							Method:      "POST",
							Path:        "/api/device/deny",
							Summary:     "Deny device authorization code",
							Auth:        "cookie",
							RequestBody: DeviceDecisionRequest{},
							Status:      http.StatusNoContent,
						},
					},
				},
				{
					Name:        "recovery",
					Description: "Password reset and account recovery",
					Routes: []Route{
						{
							Method:      "POST",
							Path:        "/api/recover",
							Summary:     "Request password recovery email",
							RequestBody: RecoverRequest{},
							Status:      http.StatusNoContent,
						},
						{
							Method:      "POST",
							Path:        "/api/recover/reset",
							Summary:     "Redeem recovery token to set new password",
							RequestBody: ResetPasswordRequest{},
							Status:      http.StatusNoContent,
						},
					},
				},
				{
					Name:        "admin",
					Description: "Operator and administrator management endpoints",
					Routes: []Route{
						{
							Method:       "GET",
							Path:         "/api/admin/clients",
							Summary:      "List registered OAuth/OIDC clients",
							Auth:         "cookie",
							ResponseBody: []ClientResponse{},
						},
						{
							Method:       "POST",
							Path:         "/api/admin/clients",
							Summary:      "Register a new client application",
							Auth:         "cookie",
							RequestBody:  CreateClientRequest{},
							ResponseBody: ClientCreatedResponse{},
							Status:       http.StatusCreated,
						},
						{
							Method:       "POST",
							Path:         "/api/admin/clients/{clientID}/secret",
							Summary:      "Rotate client secret",
							Auth:         "cookie",
							PathParams:   []Field{{Name: "clientID", Type: "string", Description: "Client ID"}},
							ResponseBody: RotateSecretResponse{},
						},
						{
							Method:       "GET",
							Path:         "/api/admin/users",
							Summary:      "List all registered users",
							Auth:         "cookie",
							ResponseBody: []UserResponse{},
						},
						{
							Method:       "POST",
							Path:         "/api/admin/users",
							Summary:      "Create a new user directly",
							Auth:         "cookie",
							RequestBody:  CreateUserRequest{},
							ResponseBody: UserResponse{},
							Status:       http.StatusCreated,
						},
						{
							Method:     "POST",
							Path:       "/api/admin/users/{userID}/disable",
							Summary:    "Disable a user account",
							Auth:       "cookie",
							PathParams: []Field{{Name: "userID", Type: "string", Description: "User ID"}},
							Status:     http.StatusNoContent,
						},
						{
							Method:       "GET",
							Path:         "/api/admin/users/{userID}",
							Summary:      "One user with their access to every app and their spaces",
							Auth:         "cookie",
							PathParams:   []Field{{Name: "userID", Type: "string", Description: "User ID"}},
							ResponseBody: UserResponse{},
						},
						{
							Method:       "GET",
							Path:         "/api/admin/clients/{clientID}/access",
							Summary:      "An app's access policy and the people granted to it",
							Auth:         "cookie",
							PathParams:   []Field{{Name: "clientID", Type: "string", Description: "Client ID"}},
							ResponseBody: ClientAccessResponse{},
						},
						{
							Method:       "PUT",
							Path:         "/api/admin/clients/{clientID}/access",
							Summary:      "Set who may sign in to an app: everyone or assigned",
							Auth:         "cookie",
							PathParams:   []Field{{Name: "clientID", Type: "string", Description: "Client ID"}},
							RequestBody:  AccessPolicyRequest{},
							ResponseBody: AccessPolicyRequest{},
						},
						{
							Method:  "PUT",
							Path:    "/api/admin/clients/{clientID}/grants/{userID}",
							Summary: "Give a user access to an app with roles, or change their roles",
							Auth:    "cookie",
							PathParams: []Field{
								{Name: "clientID", Type: "string", Description: "Client ID"},
								{Name: "userID", Type: "string", Description: "User ID"},
							},
							RequestBody:  GrantRequest{},
							ResponseBody: GrantResponse{},
						},
						{
							Method:  "DELETE",
							Path:    "/api/admin/clients/{clientID}/grants/{userID}",
							Summary: "Remove a user's access to an app",
							Auth:    "cookie",
							PathParams: []Field{
								{Name: "clientID", Type: "string", Description: "Client ID"},
								{Name: "userID", Type: "string", Description: "User ID"},
							},
							ResponseBody: OKResponse{},
						},
						{
							Method:       "GET",
							Path:         "/api/admin/events",
							Summary:      "The admin activity log, newest first, fifty at a time",
							Auth:         "cookie",
							QueryParams:  []Field{{Name: "before", Type: "integer", Description: "Return events older than this id"}},
							ResponseBody: []AdminEventResponse{},
						},
						{
							Method:       "GET",
							Path:         "/api/admin/invites",
							Summary:      "List pending user invitations",
							Auth:         "cookie",
							ResponseBody: []InviteResponse{},
						},
						{
							Method:       "POST",
							Path:         "/api/admin/invites",
							Summary:      "Create a new user invitation link",
							Auth:         "cookie",
							RequestBody:  CreateInviteRequest{},
							ResponseBody: InviteResponse{},
							Status:       http.StatusCreated,
						},
						{
							Method:       "GET",
							Path:         "/api/admin/keys",
							Summary:      "Get signing key rotation status",
							Auth:         "cookie",
							ResponseBody: KeyStatusResponse{},
						},
						{
							Method:       "POST",
							Path:         "/api/invites/redeem",
							Summary:      "Redeem an invitation and create user account",
							RequestBody:  RedeemInviteRequest{},
							ResponseBody: UserResponse{},
						},
					},
				},
			},
		},
	}
	cfg.Registry.Modules = append(cfg.Registry.Modules, signupModules()...)
	return cfg
}
