# Changelog

All notable changes to Registre are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
