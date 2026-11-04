# Changelog

All notable changes to Registre are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.5.0] - 2026-11-04

### Added

- Public self-service sign-up, "Create your Facile account", behind `SIGNUP_ENABLED` (off by
  default): an address plus an ALTCHA proof of work (verified with `altcha-lib-go`, single use,
  10 minutes), the same "check your inbox" answer and timing for known and unknown addresses (a
  known one is mailed "sign in instead"), a 30-minute single-use link stored as SHA-256, then
  name, language, acceptance of the CGU and privacy policy, and a passkey or a password
- Passwords follow NIST SP 800-63B-4 everywhere: 15 to 128 characters, no composition rules, and
  every new password (sign-up, account page, reset, invitation) screened against Have I Been
  Pwned's range API (5-character prefix, padding, 3 s timeout, skipped with a warning on outage)
- Passkeys (go-webauthn): discoverable passkey sign-in on the login page, and add, rename and
  remove on `/account`; the last way into an account cannot be removed, and adding or removing
  needs a factor proven in the last 10 minutes (`/api/account/reauth`)
- Legal pages at `/legal/{cgu,cgv,privacy,legal-notice}?lang=xx` in fr, en, es and de, served
  from `/api/legal`; drafts pending legal review. Bumping a version asks for consent again at the
  next sign-in or device approval (`/consent`); nothing is enforced while the texts are drafts
- GDPR on `/account`: a JSON export of the person's Registre data, and deletion after a 30-day
  grace period they can cancel (refused for the only owner of a shared space), then back-channel
  logout and a signed `user.deleted` event, persisted and retried, to every app's sync URI, and
  `GET /api/directory/deletions` for apps to list erasures (SPEC §6)
- The person's language (fr, en, es, de) is stored on the account, chosen from Accept-Language at
  sign-up, editable on `/account`, used for every mail, and emitted as the OIDC `locale` claim
- Space invitations: adding someone to a space now sends an invitation they accept or decline on
  `/spaces`; pending invitations never reach a snapshot, invitation mail is capped, and an
  invitation dies with its space
- Sign-up started from an app finishes that app's login, but only for the browser that started it
  and finishes it (the flow cookie from v0.4.1); a link opened elsewhere lands on Registre's home
- Rate limits per address, per email and per session on every new route; audit rows for sign-up,
  verification, consent, passkeys and deletion
- Migrations `00015_signup` (backfills every existing account as verified) and
  `00016_space_invitations`

### Changed

- The password floor is 15 characters on every path (account page, reset, invitation)
- `POST /api/spaces/{id}/members` answers 202 `{"status":"invited"}` and creates a pending
  invitation instead of a membership

## [0.4.1] - 2026-11-04

### Security

- An auth request id is no longer enough to collect a code. Before, a signed-in human who
  followed a link to `GET /api/login/session?auth_request=<id>` finished a flow an attacker had
  started, and the attacker then called `/oauth/v2/authorize/callback?id=<id>` and signed in to
  the app as that human. `/authorize/callback` now issues a code only to the browser holding both
  the per-flow cookie `/authorize` set and the account session that finished the flow, and only
  once per flow: a second callback is refused instead of overwriting the code
- Finishing a flow with an existing session is `POST /api/login/session` with the
  `X-Facile-CSRF` header and the id in the body. A GET that names `auth_request` answers 403
- Migration `00014_auth_request_browser`. Sign-ins in flight during the deploy fail once at the
  callback and work on retry

## [0.4.0] - 2026-11-04

### Added

- `PUT /api/directory/entitlements`: Ardoise pushes every space's and person's plan
  entitlements as one full replacement, guarded by a revision (409 when stale), validated
  strictly (at most 10,000 scopes, revision at most 1,000,000 ahead), and audited in the
  same transaction. Only the client named by `ENTITLEMENTS_WRITER` may call it
- The directory snapshot carries `entitlements_revision`, `entitlements` on each space and a
  `people` list with each person's personal entitlements (porte `directory` wire types)
- A plan section on each space page: members see the plan, owners see the published catalog,
  upgrade through Stripe checkout and open the billing portal. Needs `ARDOISE_BILLING_URL`
  and a `BILLING_API_TOKEN` of at least 32 characters; hidden otherwise. The catalog is cached
  60 s and the plan routes are rate limited per person
- Migration `00013_entitlements`

## [0.3.0] - 2026-11-04

### Added

- Glouton can exchange a user's token for Bulle (`bulle.read`, `bulle.write`) so Bulle writes mail inside Glouton

## [0.2.0] - 2026-11-03

### Added

- Bulle can exchange a user's token for Glouton (`glouton.read`, `glouton.write`)
- Glouton can exchange a user's token for Courrier (`courrier.read`, `courrier.write`) to send outreach and read replies

## [0.1.0] - 2026-11-03

First tagged release of the Facile Suite's identity provider.

### Added

- OIDC provider: authorization code with PKCE, `client_credentials` machine tokens,
  the device authorization grant, and RFC 8693 token exchange for delegated app tokens
- RS256 tokens published through a rotating JWKS, plus introspection, revocation and
  back-channel logout
- Human accounts: password sign-in, TOTP second factor, account recovery, a tested
  break-glass path, and profile pictures emitted as the `picture` claim
- Per-app access: decide who may sign in to each app and with which roles, filtered
  into the `roles` claim
- Suite spaces owned by Registre: create, rename, manage members, leave and delete at
  `/spaces`, with every app nudged after a change
- Admin dashboard for users, apps, clients and access
- Muse-based client embedded in the API binary: landing page, app portal and device
  verification page
- Declarative YAML seed that registers every suite app as an OIDC client
- Browser error reporting into Journal

### Changed

- Spaces use the share-circle icon (muse v0.6.6)

[Unreleased]: https://github.com/FacileStudio/Registre/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/FacileStudio/Registre/releases/tag/v0.1.0
