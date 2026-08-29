# Registre — API Reference

HTTP endpoints and OpenID Connect / OAuth 2.0 protocol routes provided by the Registre identity service.

The interactive documentation is served live at `/docs` (powered by Scalar), and the OpenAPI 3.1 schema is available at `/docs/openapi.json`.

## Endpoints

### OpenID Connect & OAuth 2.0 (`oidc`)

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/.well-known/openid-configuration` | Public | Discovery document reporting endpoints and supported capabilities |
| `GET` | `/oauth/v2/keys` | Public | JSON Web Key Set (JWKS) containing active public signing keys |
| `POST` | `/oauth/v2/token` | Basic / Client Secret | Issue access tokens, refresh tokens, and ID tokens |
| `POST` | `/oauth/v2/introspect` | Bearer / Client Auth | Inspect active status and metadata of a token (RFC 7662) |
| `POST` | `/oauth/v2/revoke` | Bearer / Client Auth | Revoke an issued token (RFC 7009) |
| `GET` | `/oauth/v2/userinfo` | Bearer | Return claims for the authenticated user (OIDC UserInfo) |
| `POST` | `/oauth/v2/userinfo` | Bearer | Return claims for the authenticated user via POST |
| `GET` | `/oauth/v2/authorize` | Public | Interactive browser authorization endpoint |
| `POST` | `/oauth/v2/authorize/callback` | Public | Authorization callback endpoint |
| `GET` | `/oauth/v2/sessions/end` | Cookie | RP-initiated logout endpoint |
| `POST` | `/oauth/v2/sessions/end` | Cookie | RP-initiated logout endpoint via POST |
| `POST` | `/oauth/v2/device` | Client Auth | Start device authorization flow for terminal clients (RFC 8628) |

### Login (`login`)

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/login` | Public | Authenticate with email and password |
| `POST` | `/api/login/totp` | Public | Complete second factor TOTP challenge |
| `GET` | `/api/config` | Public | Browser client configuration for frontend logging |

### Account (`account`)

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/account/` | Cookie | Fetch profile and security status for authenticated user |
| `POST` | `/api/account/password` | Cookie | Change current user password |
| `POST` | `/api/account/totp/begin` | Cookie | Start TOTP registration and generate QR code/secret |
| `POST` | `/api/account/totp/confirm` | Cookie | Verify initial TOTP code to activate two-factor authentication |
| `POST` | `/api/account/totp/disable` | Cookie | Disable TOTP on account |
| `POST` | `/api/account/totp/recovery-codes` | Cookie | Regenerate backup recovery codes |
| `POST` | `/api/account/logout` | Cookie | Log out account session |

### Device Flow (`device`)

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/device/authenticate` | Public | Initiate device approval sign-in round trip |
| `GET` | `/api/device/` | Cookie | Inspect pending device authorization request by user code |
| `POST` | `/api/device/approve` | Cookie | Authorize device flow |
| `POST` | `/api/device/deny` | Cookie | Deny device flow |

### Recovery (`recovery`)

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/recover` | Public | Request password recovery email |
| `POST` | `/api/recover/reset` | Public | Reset account password using recovery token |

### Admin (`admin`)

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/admin/clients` | Cookie (Admin) | List registered OAuth/OIDC clients |
| `POST` | `/api/admin/clients` | Cookie (Admin) | Register a new OAuth/OIDC client application |
| `POST` | `/api/admin/clients/{clientID}/secret` | Cookie (Admin) | Rotate client secret |
| `GET` | `/api/admin/users` | Cookie (Admin) | List all registered accounts |
| `POST` | `/api/admin/users` | Cookie (Admin) | Create a new user account directly |
| `POST` | `/api/admin/users/{userID}/disable` | Cookie (Admin) | Disable user account |
| `GET` | `/api/admin/invites` | Cookie (Admin) | List pending invitations |
| `POST` | `/api/admin/invites` | Cookie (Admin) | Generate a new user invitation link |
| `GET` | `/api/admin/keys` | Cookie (Admin) | Inspect signing key rotation status |
| `POST` | `/api/invites/redeem` | Public | Accept invitation and complete registration |
