# Changelog

All notable changes to Registre are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.13.1] - 2026-11-11

### Removed

- Bulle may no longer exchange a user's token for Casier. An assistant must not handle a person's secrets, and v0.12.1 granted it by mistake. Casier keeps its MCP resource, so third-party clients a person connects can still reach it.

## [0.13.0] - 2026-11-11

### Changed

- The landing header, footer, sign-in screens and legal pages show the catalog `solar:key-square-bold-duotone` mark, registered in `src/lib/icons.ts`, instead of the shield and the old key glyph.

### Fixed

- The favicon, touch and manifest icons, link preview image and web manifest come from the muse v0.19.0 brand generator, and the head carries the suite's canonical title, description, theme colours and Open Graph and Twitter tags. The signed-out landing title matches the suite catalog.

## [0.12.4] - 2026-11-11

### Added

- Toile, the suite's whiteboard at `https://toile.facile.studio`, is a registered application open to every account, with its MCP resource at `https://toile.facile.studio/mcp` (`toile.read`, `toile.write`). Toile may exchange a user's token for Nuage, Opus, Docs, Feuilles, Agenda and Bulle with read and write, and Bulle may exchange one for Toile. Its secret comes from `REGISTRE_SEED_TOILE_SECRET`, which the compose file now forwards.

## [0.12.3] - 2026-11-11

### Fixed

- An unknown first argument, such as `registre help`, prints the usage to stderr and exits 2 instead of starting a second server. With no argument the binary still runs the migrations and serves, and `user-create`, `user-disable`, `user-erase`, `break-glass`, `migrate` and `healthcheck` are unchanged.

### Added

- A `test` CI job runs `scripts/check.sh` against a `postgres:16` service, so the database tests, the wire goldens in `apps/api/testdata/wire` among them, run on every push and pull request. The wire tests pin the process zone to UTC, the zone of the production image, so `directory.golden` records `deleted_at` as production sends it rather than in the recording machine's offset.

## [0.12.2] - 2026-11-11

### Added

- `registre user-erase --email <address>` erases one account at once, through the same erasure the grace-period sweeper and the dashboard delete run: sessions end, the account goes, and every suite app gets its back-channel logout and deletion notice. It takes exactly one bare address, prints only `erased`, `not found` or `failed`, and sends no farewell mail. It is meant for throwaway smoke accounts.

## [0.12.1] - 2026-11-11

### Added

- Nine more MCP servers are registered resources, each at `https://<app>.facile.studio/mcp` with `<app>.read` and `<app>.write`: Mycelium, Glouton, Plume, Casier, Capsule, Journal, Antenne, Vision and Echo.
- Bulle may exchange a user's token for Plume, Casier, Capsule, Journal, Antenne, Vision and Echo with read and write. Its existing grants for Mycelium and Glouton already cover their new resources.
- Added Feuilles (`https://feuilles.facile.studio`) to the Suite apps grid on the home dashboard.

## [0.12.0] - 2026-11-11

### Changed

- Go 1.26.11 (builder image `golang:1.26.11-alpine`), tronc v0.21.0, porte v0.18.3 and porte/directory v0.4.0. porte's password hashing and spaces ladder are byte-identical across the range, and the directory wire types are unchanged.
- Every response carries tronc's security headers: a report-only Content-Security-Policy, an enforced `frame-ancestors 'none'; object-src 'none'; base-uri 'self'`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy`, a `Permissions-Policy` denying camera, microphone and geolocation, and HSTS behind a trusted proxy. `Cross-Origin-Opener-Policy` is `unsafe-none`, so a sign-in opened in a popup keeps its opener.
- The per-address rate limits use tronc's `/64` key and remember at most 50,000 addresses.
- `golang.org/x/crypto` v0.56.0, `golang.org/x/text` v0.41.0 and `go-jose` v4.1.6, clearing what douane reports.
- filet v0.19.3 with `preset: facile` (same findings as before), `filet gate .` in the pre-push hook beside `scripts/check.sh`, and a douane job in CI (`-fail critical -scope prod`, one dated permit in `douane.yml`).

### Fixed

- The staticcheck findings `filet gate` reports: an unused admin helper, two unused test values, and four literal U+202E characters in tests, now escaped.

### Added

- Golden tests pin discovery, the key set, the token endpoint, userinfo, introspection, the directory snapshot, the deletion feed and the signed `user.deleted` push (`apps/api/testdata/wire`).

## [0.11.0] - 2026-11-10

### Added

- Space invitation mails name who invited you, the space, the role and the expiry date, in your language, with a "View invitation" button. Names are cleaned (no control or direction-override characters, 80 characters at most) and escaped, and every mail now has a plain-text part that spells out its link. The anti-abuse caps are unchanged.
- An invitation page at `/invitations/{id}`: inviter, space, role and expiry with Accept and Decline. Opening it changes nothing; only the POSTs answer. It says clearly when the invitation is expired, withdrawn, or for another account, and signs you in with `?next=` back to it. `GET /api/spaces/invitations/{id}` drives it.
- The join mail for an address without an account opens sign-up with the invitation attached, so the new account lands on the invitation.
- `GET /api/directory/invitations?sub=` (HTTP Basic, first-party application clients) answers `{count, invitations[{id, space_name, role, invited_by, expires_at, url}]}` for a suite app's user menu. See docs/invitations.md.

### Changed

- Licensed under the GNU Affero General Public License v3.0 only (`AGPL-3.0-only`). See `LICENSE`.
- Expired invitations are kept 30 days so their page can say "expired".
- The directory routes refuse public and third-party clients outright.

## [0.10.7] - 2026-11-10

### Added

- Opus's MCP server is a registered resource: `https://opus.facile.studio/mcp` with scopes `opus.read` and `opus.write`.

## [0.10.6] - 2026-11-10

### Added

- Sablier's MCP server is a registered resource: `https://sablier.facile.studio/mcp` with scopes `sablier.read` and `sablier.write`.

## [0.10.5] - 2026-11-10

### Added

- Bulle may exchange a user's token for Sonar (`sonar.read`), so it can call Sonar's MCP server for web search and fetch.

## [0.10.4] - 2026-11-10

### Added

- Nuage's MCP server is a registered resource: `https://nuage.facile.studio/mcp` with scopes `nuage.read` and `nuage.write`.

## [0.10.3] - 2026-11-10

### Added

- Access tokens bound to an MCP resource (browser flow, refresh, and Bulle's exchange with `resource`) carry the claims an app's web login creates the account from: `email` with `email_verified: true` (only for a verified address), `name`, `picture` and `locale`. porte/mcpauth v0.2.0 uses them to create the local account at the first MCP call, so a new Registre user no longer has to open an app on the web before using its MCP server. A disabled person gets none; plain access tokens are unchanged. See `docs/mcp.md`.

## [0.10.2] - 2026-11-10

### Added

- Sonar declares its MCP server: resource `https://sonar.facile.studio/mcp` with the one scope `sonar.read`, since search and fetch are reads.

## [0.10.1] - 2026-11-10

### Changed

- Agenda, Opus, Sablier, Nuage and Mycelium may exchange a signed-in user's token for audience `bulle` with `bulle.read` and `bulle.write`, like Courrier and Glouton, so each app's Ideas page can call Bulle as that user.

## [0.10.0] - 2026-11-10

### Added

- MCP authorization, second half (see `docs/mcp.md`): third-party MCP clients such as Claude, ChatGPT, Cursor and VS Code.
- Client ID metadata documents (CIMD): a `client_id` that is an https URL is fetched through an SSRF guard (public addresses only, no redirects, 5 s, 16 KiB), must name itself as `client_id`, and is cached per `Cache-Control` (5 min to 24 h). AS metadata advertises `client_id_metadata_document_supported`.
- Dynamic client registration (RFC 7591) at `POST /register` for public clients, `application_type` defaulting to native, 20 per address then one a minute.
- A consent screen at `/connect`: a third-party client gets a code only after the person allows it. It shows the redirect host, the app and the scopes, and warns when every redirect is on the person's own machine.
- Connected apps in the account (`/account/apps`): list and disconnect. Disconnecting ends the client's refresh tokens at once (migration 00022 adds `client_grants`).
- Third-party clients get 10-minute access tokens and must name a registered MCP resource.

### Fixed

- Deleting a user from the admin dashboard now runs the same erasure as a person's own deletion: sessions ended, back-channel logout, spaces handed over, and one deletion notice per suite app (pushed to its `deletion_uri` or listed for polling). Before, apps kept orphaned accounts.
- Deletion notices are queued for suite apps only, never for third-party clients.
- `/logout` and `/api/logout` answer GET and POST only and are documented, so the API reference test passes again.

## [0.9.2] - 2026-11-09

### Added

- Added Place (`https://place.facile.studio`) with its SVG logo to the Suite apps grid on the home dashboard.

## [0.9.1] - 2026-11-09

### Fixed

- Added missing Tailwind radius scale and utility classes (`rounded-2xl`, `rounded-3xl`, `rounded-fc-*`) so cards and containers have rounded corners.
- Fixed button height next to inputs and selects by applying `size="lg"` across passkey, space, and member forms.
- Fixed 404 error on logout by calling `account.logout()` and wiring GET fallback handlers on `/logout` and `/api/logout`.
- Harmonized recover, reset, and redeem views with `@facile/muse` components.

## [0.9.0] - 2026-11-09

### Changed

- Harmonized the user interface with `@facile/muse` v0.16.1 and stripped decorative border lines and dividers across all views.
- Redesigned the SSO login page into a clean, centered Google-style authentication card.
- Streamlined CLI OAuth / device authorization (`/device`): automatic code detection on load without manual entry, clean confirmation card, and seamless return to device flow after sign-in.
- Redesigned the public landing page with modern muse `LandingHeader` and `LandingFooter` components, sticky floating header, and borderless feature cards.

### Added

- Sign in with GitHub via OAuth 2.0 with PKCE (S256), primary verified email resolution, and account linking under security settings.
- Emits the `connections.github` claim in OIDC userinfo.
- Environment credentials `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET`.

## [0.7.2] - 2026-11-08

### Removed

- Removed the studio vitrine (Facile) from the apps list.

## [0.7.1] - 2026-11-08

### Added

- Sonar is now listed in the suite directory on the user dashboard.

### Changed

- Removed Coffre, Gare, Abeille, and Miel from the user-facing apps list.

## [0.7.0] - 2026-11-08

### Added

- The `sonar` client in the seed, with its compose secret forwarding (PR #12).
- MCP authorization, first half (see `docs/mcp.md`). `GET /.well-known/oauth-authorization-server` serves RFC 8414 metadata. `/authorize`, the code and refresh grants and token exchange accept the RFC 8707 `resource`: the access token's `aud` becomes the resource URI alone and it carries a space-separated `scope` claim with the resource's scopes. Unknown resources, two resources, or another app's scopes answer `invalid_target`.
- Applications declare their MCP server in the seed with an `mcp` block (resource URI and `<app>.read`/`<app>.write` scopes; migration 00021). Courrier (`https://courrier.facile.studio/mcp`) and Agenda (`https://agenda.facile.studio/mcp`) are the first two.
- Token exchange accepts `resource` and maps it to the app that declares it, so Bulle's existing grant for an app covers its MCP server.
- Every authorization response carries `iss` (RFC 9207), and both discovery documents advertise it.
- CORS from any origin, without credentials, on discovery, AS metadata, JWKS, token and revocation.
- Token requests without a client secret get their own per-address budget, 20/s burst 200 (`REGISTRE_PUBLIC_TOKEN_RATE`, `REGISTRE_PUBLIC_TOKEN_BURST`), so MCP connectors sharing an egress range are not throttled together.

### Security

- `code_challenge_method=plain` is refused with `invalid_request`.
- A token for an app's MCP server follows that app's access policy, at sign-in, on refresh and on exchange.
- Every HTML page answers `X-Frame-Options: DENY` and `frame-ancestors 'none'`.

### Changed

- Discovery advertises `response_types_supported: ["code"]`, the only response type clients may use.

## [0.6.8] - 2026-11-08

### Added

- `GET /version` and `GET /api/version` answer `{"version":"vX.Y.Z"}`, public and cheap. The image build links the value in from the top released heading of `CHANGELOG.md` (`scripts/version.sh`), so Sonde can check the deployed version

## [0.6.7] - 2026-11-07
### Security

- Failed password logins are throttled per account, stored in Postgres (migration 00020). The first five attempts are free, then each attempt waits 30 s, doubling to a 15 min cap, and a success resets the count. A throttled attempt answers 429 `too_many_attempts` with `Retry-After`, with the same status and body for unknown and known addresses. There is no hard lockout. The TOTP step has its own per-account throttle, so a fresh password login cannot reset it.
- Argon2 verification queues for at most 3 s (`REGISTRE_VERIFY_QUEUE_WAIT`) and then answers 503 with `Retry-After`, on `/api/login`, the token endpoint and the directory API, instead of waiting without limit. The default concurrency is now two per core.
- The per-address login budget drops from 5/s to 1/s (burst 10), and the token endpoint gets the same budget. Both are configurable with `REGISTRE_LOGIN_RATE`, `REGISTRE_LOGIN_BURST`, `REGISTRE_TOKEN_RATE` and `REGISTRE_TOKEN_BURST`.

### Changed

- The database pool is capped: 40 open and 10 idle connections, recycled after 30 min (`REGISTRE_DB_MAX_OPEN_CONNS`, `REGISTRE_DB_MAX_IDLE_CONNS`, `REGISTRE_DB_CONN_MAX_LIFETIME`). At 128 concurrent requests the uncapped pool exhausted Postgres `max_connections`.
- The login page shows a specific message on 429 and 503.

## [0.6.6] - 2026-11-07

### Added

- `deletion_uri` in the seed for Glouton.

## [0.6.5] - 2026-11-07

### Added

- `GET /api/directory/deletions` pages with an opaque `cursor` (created_at and id) and answers `next_cursor` when more notices remain, so more than 1000 notices on one timestamp can be listed. Callers sending only `since` still get the first 1000.
- `deletion_uri` in the seed for Agenda, Vision, Casier, Journal, Antenne, Mycelium, Plume, Prisme, Sablier, Bulle, Courrier and Opus.

### Security

- A social sign-in that owes a TOTP code no longer puts the challenge token in the `/login` URL. It travels in an HttpOnly, SameSite=Strict `__Host-` cookie, and the page posts the code alone.

## [0.6.4] - 2026-11-07

### Added

- Seed client `echo` for echo.facile.studio (sign-in, directory sync and GDPR deletion URIs), with its `REGISTRE_SEED_ECHO_SECRET` forwarded in docker-compose.yml.

## [0.6.3] - 2026-11-07

### Changed

- The production seed lets Plume exchange a signed-in user's token for Nuage (`nuage.write` only). Plume uses it to store signed PDFs in the user's private Plume folder in Nuage.
  Ardoise needs the same entry; its client is not in the seed (see the Nuage cutover runbook).

## [0.6.2] - 2026-11-06

### Security

- The `connections` claim reaches only clients that opted into the provider through `login_providers`, in both the ID token and userinfo. It lists only those providers. A client that opted into none gets no claim, whatever scope it requested
- The GDPR export now includes `social_identities`: provider, provider subject, display name and `linked_at`. Tokens are never exported

### Added

- Audit entries `account.connection_added` on every social link and `account.created_social` when a social sign-in creates an account, written in the same transaction as the link

## [0.6.1] - 2026-11-06

### Added

- Deep-link linking. An app sends a person to `/account/security?link=discord&return_to=<url-encoded>`
  and Registre links the provider and sends the browser back with `connection=linked|taken|failed`
  merged into `return_to`'s query, with no manual step. A stale factor opens the usual
  reauthentication modal and resumes. A provider that is already linked goes straight back
- `POST /api/account/connections/{provider}/link` takes an optional JSON body
  `{"return_to": "<url>"}`. It is accepted only when it is https (http in development), has no
  userinfo, and matches the origin of a redirect URI of an enabled client whose `login_providers`
  names the provider. Anything else is a 400 `invalid_return_to`. The URL is checked again before
  the redirect. Migration `00019_social_return.sql` adds `social_states.return_to`
- `GET /api/account/connections/{provider}/return?return_to=` answers `{"url"}` for an already
  linked provider and a valid `return_to`, otherwise 400 `invalid_return_to`
- A canceled or failed link now also returns to `return_to` with `connection=failed`

### Changed

- Connections section: one tile per provider with the Discord logo, a status badge and a primary
  action

## [0.6.0] - 2026-11-06

### Added

- Sign in with Discord. A client opts in with `login_providers: [discord]` in the seed, and only
  then does its login page show "Continue with Discord". Registre sends the browser to Discord
  (authorization code with PKCE S256, `identify email`), takes it back at
  `/api/login/social/discord/callback`, and resolves the profile: a linked Discord account signs
  its user in; a verified Discord email equal to an existing address links to that account; any
  other verified email creates a passwordless account, only while public sign-up is open. A
  disabled user is refused, and an account with two-step sign-in still owes the code
  (`/login?auth_request=&challenge=`). The state is stored hashed, single use, and bound to the
  browser by an HttpOnly cookie. `DISCORD_CLIENT_ID` and `DISCORD_CLIENT_SECRET` turn it on, both
  or neither
- `connections` scope and claim: `{"discord": "<discord user id>"}` in the ID token and userinfo
  when the scope is granted, so an app can map a Discord member to its account
- Account page "Connections" (security tab): link and unlink a Discord account behind a fresh
  factor; an unlink that would leave no password, passkey or other linked account is refused. The
  section lists only providers a client opted into
- Migration `00018_social.sql`: `social_identities`, `social_states` and `clients.login_providers`
- Seed: `place` opts into Discord and asks for the `connections` scope

## [0.5.1] - 2026-11-06

### Security

- Deletion events no longer go to `directory_sync_uri`, whose porte handler answers 202 to any
  POST, so notices were counted delivered while nothing erased. They are pushed only to a new
  per-client `deletion_uri` (seed and dashboard field), delivered only on a 2xx from it, and
  `GET /api/directory/deletions` is the source of truth. SPEC §6 lists what a receiver verifies
  (`typ` `secevent+jwt`, signature, `iss`/`aud`/`exp`, the event key, `jti`)
- Freshness for passkey changes and deletion requests is an explicit `sessions.factor_proven_at`,
  not a session's creation time, so the session a password change reissues proves nothing; a
  password change now also wants the TOTP or recovery code when two-step sign-in is on
- Accepting a space invitation needs the inviter still in the space at the offered rank, and
  removing or demoting an inviter withdraws their pending invitations
- `POST /api/login`, `/api/login/totp`, `/api/login/passkey/finish` and `/api/signup/finish`
  require `X-Facile-CSRF` (login CSRF)
- Invitation mails carry no inviter or space text at all, and one inviter's invitations send at
  most 20 mails a day
- Erasure scrubs the person's address from `admin_events`; a sign-up passkey ceremony is bound to
  the link it was begun for; a login an access policy refuses leaves no session row; every
  address rate limit counts IPv6 per /64
- `POST /api/spaces/{id}/billing/portal` sends Ardoise the signed-in owner's email from the
  session, as checkout does, for Ardoise's owner check
- The dashboard's client edit writes only the columns its form owns, and of those only the fields
  the request carried: an omitted field keeps its stored value, so saving `facile-cli` no longer
  replaces its grant types (device grant included) with defaults. It used to write the
  whole declaration, which turned the public `facile-cli` client confidential and zeroed its token
  lifetime on save; `deletion_uri` equal to `directory_sync_uri` is refused there and in the seed
- An invitation from a disabled inviter can no longer be accepted
- Migration `00017_review_v051`. Rolling back to v0.5.0 needs its Down first: `registre migrate`
  has no down subcommand, so run the `-- +goose Down` block of `00017_review_v051.sql` with psql
  and `DELETE FROM goose_db_version WHERE version_id = 17`. v0.5.0 reads
  `sessions.reauthenticated_at`, which 00017 renames to `factor_proven_at`

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
