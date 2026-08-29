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

func Reference() apiref.Config {
	return apiref.Config{
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
							Summary:      "Logout account session",
							Auth:         "cookie",
							ResponseBody: map[string]any{"logged_out": true},
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
}
