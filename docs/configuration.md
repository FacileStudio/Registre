# Configuration

Everything is read from the environment at boot. Every check that can be made without a database
is made there and then: an identity provider that starts and then cannot sign is worse than one
that refuses to start.

## Required

| Variable | What it is |
|---|---|
| `DATABASE_URL` | PostgreSQL connection string. `docker-compose.yml` builds it from `POSTGRES_*` |
| `REGISTRE_ISSUER` | The URL registre answers as, stamped into every token. Must be `https` unless `APP_ENV=development`, and must carry no query or fragment |
| `ENCRYPTION_KEY` | Base64 32-byte root key. `openssl rand -base64 32` |

`ENCRYPTION_KEY` seals every signing key in the database. Losing it means every token registre
ever issued stops verifying and every app has to be pointed at a fresh instance. It belongs in
casier, not in a file next to the compose stack.

`REGISTRE_ISSUER` is the same value every app in the suite carries as `OIDC_ISSUER`. Changing it
invalidates everything already issued, in every app, at once.

## The token and key lifecycle

| Variable | Default | What it decides |
|---|---|---|
| `REGISTRE_ACCESS_TOKEN_TTL` | `1h` | How long a machine token lives. A client row may set its own |
| `REGISTRE_REFRESH_TOKEN_TTL` | `720h` | How long a refresh grant may be redeemed. It rotates on every redemption |
| `REGISTRE_KEY_ROTATION_INTERVAL` | `720h` | How long a key signs before it is replaced |
| `REGISTRE_KEY_PUBLISH_DELAY` | `15m` | How long a new key sits in the JWKS before it signs |
| `REGISTRE_KEY_RETENTION` | `2 ×` the token lifetime | How long a retired public key stays in the JWKS |

Two relationships are enforced at boot rather than discovered in production:

- **`REGISTRE_KEY_RETENTION` must be at least `REGISTRE_ACCESS_TOKEN_TTL`.** Below it, a retired
  key leaves the JWKS while tokens it signed are still valid, and every app holding one 401s.
- **`REGISTRE_KEY_PUBLISH_DELAY` must be shorter than `REGISTRE_KEY_ROTATION_INTERVAL`.** Above
  it, the next key is published before the current one has begun signing and the state machine
  never settles.
- **`REGISTRE_REFRESH_TOKEN_TTL` must be longer than `REGISTRE_ACCESS_TOKEN_TTL`.** A grant that
  dies before the thing it renews is a login that breaks for no reason.

The publish delay has to outlast the key set cache of the slowest relying party in the suite.
`go-oidc`, which every app uses through porte, refetches on an unknown `kid` and so recovers on
its own; fifteen minutes is the margin for anything that does not.

`docker-compose.yml` leaves `REGISTRE_KEY_RETENTION` empty on purpose. Pinning a literal there
would break the derived default silently the moment somebody raised the token lifetime alone: the
boot guard would refuse to start, correctly, and the reason would be a value nobody set.

## Pool and per-address limits

| Variable | Default | What it decides |
|---|---|---|
| `REGISTRE_DB_MAX_OPEN_CONNS` | `40` | Open database connections; keep well under Postgres `max_connections` |
| `REGISTRE_DB_MAX_IDLE_CONNS` | `10` | Idle connections kept warm (clamped to the open cap) |
| `REGISTRE_DB_CONN_MAX_LIFETIME` | `30m` | Age after which a connection is recycled |
| `REGISTRE_LOGIN_RATE` / `REGISTRE_LOGIN_BURST` | `1` / `10` | Per-address budget on the login endpoints, requests per second |
| `REGISTRE_TOKEN_RATE` / `REGISTRE_TOKEN_BURST` | `1` / `10` | Per-address budget on the token endpoint for requests that carry a client secret |
| `REGISTRE_PUBLIC_TOKEN_RATE` / `REGISTRE_PUBLIC_TOKEN_BURST` | `20` / `200` | Per-address budget on the token endpoint for public clients (no secret, no argon2). MCP connectors such as claude.ai share Anthropic's egress range, so this one is large |

A password check costs about 145 ms of CPU, so two cores verify about 13 logins a second. The
per-address budgets are set to one request a second so that a handful of addresses cannot saturate
the box. The suite's apps reach the token endpoint through one or a few addresses: raise
`REGISTRE_TOKEN_RATE` if they start seeing 429.

## The per-account login throttle

Each password attempt and each second-factor attempt counts against the account, stored in the
`login_throttles` table (keys are SHA-256 digests, never addresses). Five attempts are free, then
each attempt waits 30 s, doubling to 15 min. A success resets the count. A throttled attempt
answers 429 `too_many_attempts` with `Retry-After`, identically for unknown and known addresses,
and does not extend the delay. There is no hard lockout. While a delay runs, the right password is
refused too (at most 15 min); passkeys, social sign-in and the emailed reset are not affected.

## The argon2 ceiling

| Variable | Default | What it decides |
|---|---|---|
| `REGISTRE_VERIFY_CONCURRENCY` | two per core | How many argon2 verifications run at once |
| `REGISTRE_VERIFY_QUEUE_WAIT` | `3s` | How long a request queues for a slot before it is answered 503 |

**This is a memory ceiling wearing a count. Multiply it by 64 MiB.** argon2id at porte's parameters
allocates 64 MiB per call, and the token endpoint has to hash before it can refuse a bad credential,
because answering an unknown client faster than a real one is an account-existence oracle. Without a
ceiling, every concurrent request is 64 MiB of registre's memory, from anyone, unauthenticated.
Measured against this code before the ceiling existed: 148 MB resident for one bad credential,
3.2 GB at fifty concurrent, 11.1 GB at two hundred.

One per core is the default because argon2 is CPU bound: hashing more values at once than the
machine has cores buys no throughput and costs 64 MiB each. It is the same default, for the same
reason, that Keycloak settled on after hitting this ([keycloak#28477](https://github.com/keycloak/keycloak/issues/28477)).

Callers past the ceiling **wait**; they are not refused. A refusal would reach the client as
`invalid_client`, because the OIDC library maps any error out of client authentication to that, and
telling an agent its credentials are wrong when the server is merely busy is worse than making it
wait. Lower this on a container with a memory limit; raise it only if you have measured a queue.

## The seed

| Variable | Default | What it decides |
|---|---|---|
| `REGISTRE_SEED_FILE` | unset | Path to the YAML bootstrap. Unset means the database is the only source of clients |

The file names environment variables, never secrets. Each service account carries a `secret_env`,
and the variable it names must be set or the boot fails saying which one is missing. See
[`seed.example.yaml`](../seed.example.yaml).

Applying is an upsert keyed on `client_id`, and it never deletes: an account dropped from the file
is logged at boot and left working.

## Password recovery

| Variable | Default | What it decides |
|---|---|---|
| `SMTP_HOST` | unset | The mail server. Absent means recovery is off |
| `SMTP_PORT` | `587` | 465 gets implicit TLS, anything else STARTTLS when offered |
| `SMTP_USER` | unset | Empty means no authentication |
| `SMTP_PASS` | unset | |
| `SMTP_FROM` | unset | The From address. Absent means recovery is off |
| `SMTP_FROM_NAME` | unset | The display name |

These are the suite-wide names, read by `facteur` and `facteur-ts` alike, so one set of variables
is documented for every app that sends mail.

**Recovery off is a supported state, not a broken one.** `POST /api/recover` and
`POST /api/recover/reset` mount whatever the configuration says. With no mailer the request route
still answers the same status and the same body it always does, so a caller cannot detect the
difference. The boot log says which state the instance is in.

**The send is deliberately asynchronous.** `internal/recovery` pays an argon2 cost on both the
known and the unknown path so the two take the same time, and only the known path has a mail to
send. A synchronous SMTP round trip would put the difference straight back as latency, which is
the same enumeration oracle wearing a stopwatch. Delivery failures reach the log rather than the
caller, which is the right trade: the caller has been told a link is on its way whether or not
their address was known, so there is no answer a failure could usefully change.

## Plans and billing

| Variable | Default | What it decides |
|---|---|---|
| `ENTITLEMENTS_WRITER` | unset | The one client id allowed to `PUT /api/directory/entitlements`, Ardoise's. Unset means the route answers 404 to everyone |
| `ARDOISE_BILLING_URL` | unset | Ardoise's API base, such as `https://api-ardoise.facile.studio`. The only host the plan section calls. Must be `https` unless `APP_ENV=development`; a malformed value refuses boot |
| `BILLING_API_TOKEN` | unset | The bearer registre sends to Ardoise, the same value Ardoise holds. Server side only, never logged |

The plan section on `/spaces/<id>` shows only when `ARDOISE_BILLING_URL` is set and
`BILLING_API_TOKEN` is at least 32 characters. A shorter token counts as absent, so a guessable
secret turns the feature off rather than weakening it. With the section off, the plan API answers
`enabled: false` and checkout and portal answer 404. Every call to Ardoise has a 5 s timeout and a
1 MiB response cap, and a checkout or portal URL is only handed to the page when its origin is
exactly `https://checkout.stripe.com` or `https://billing.stripe.com`.

The writer is checked after the usual client authentication, so the argon2 ceiling still applies:
a wrong secret is 401, another app is 403.

## Break-glass

| Variable | Default | What it decides |
|---|---|---|
| `REGISTRE_BREAK_GLASS_PASSWORD` | unset | The password `registre break-glass` sets, minimum 16 characters |

Read only by the `break-glass` subcommand, and never by the server. It is an environment variable
rather than a flag because a flag lands in shell history and in `ps` output. See
[SPEC §7](../SPEC.md) for what the command does and why it is not a route.

```sh
REGISTRE_BREAK_GLASS_PASSWORD='...' registre break-glass --email operator@facile.studio
```

## Inherited from tronc

`APP_ENV`, `PORT`, `LOG_LEVEL`, `CORS_ALLOWED_ORIGINS`, `TRUSTED_PROXIES`, `JOURNAL_URL`,
`JOURNAL_TOKEN`. `APP_ENV` decides one thing here and one thing only: whether a plain-http issuer
is tolerated. There is no separate flag for that, so it cannot be turned on in production by
setting a boolean.

`CORS_ALLOWED_ORIGINS` unset denies every cross-origin caller, which is what phase 0 wants:
machine tokens are fetched server to server and no browser calls the token endpoint yet.

## Log shipping

| Variable | Default | What it decides |
|---|---|---|
| `JOURNAL_URL` | unset | Journal's API, which must end in `/api`. On la ruche, `http://journal-api:4010/api` |
| `JOURNAL_TOKEN` | unset | The per-app key from Journal's Keys page |
| `JOURNAL_BROWSER_URL` | unset | Journal's API for the client build, also ending in `/api` |
| `JOURNAL_BROWSER_KEY` | unset | The public, origin-locked key the browser reports with |

The browser pair is not the server pair. `JOURNAL_URL` and `JOURNAL_TOKEN` are the Go SDK's
secret credential; `JOURNAL_BROWSER_URL` and `JOURNAL_BROWSER_KEY` are handed to the client over
`GET /api/config` and ship in a response to any visitor, so they must be the public kind of key.
Serving them rather than baking them into the bundle is what makes rotating one an environment
change instead of a rebuild.

**Three boot-refusing rules apply to the browser pair**, all in `internal/env`: both must be set
or neither, the URL must end in `/api`, and the same `/api` rule the server pair has. A URL
without the suffix is answered by Journal's own SPA catch-all with `200` and a page of HTML,
which the reporting client reads as success, so the reports disappear with no error anywhere.

Both or neither: with one missing, nothing ships and nothing says so.

**This is the suite's auth audit trail, so treat it as required in production.** registre's own lines
are the key lifecycle and the seed; the ones that matter more come from `zitadel/oidc`, which logs
every refusal it issues — an unknown client, a scope the register does not grant, an unsupported
grant. Those reach Journal only because `main` calls `slog.SetDefault`: `op.WithLogger` accepts a
logger and discards it, so the library emits through the package-level slog and nothing else can
intercept it. Remove that call and the provider's refusals silently revert to slog's own text
format, at slog's own level, outside Journal.

Emitting to `antenne` is a separate question and still open — SPEC §10 question 5.

## Sign-up, passkeys and legal documents

| Variable | Default | What it decides |
|---|---|---|
| `SIGNUP_ENABLED` | `false` | Opens public sign-up. Deploying never opens it by surprise: the owner sets it |
| `ALTCHA_HMAC_KEY` | unset | HMAC key the ALTCHA proof-of-work challenges are signed under. Shorter than 32 bytes counts as absent and keeps sign-up off, with an error in the boot log |
| `WEBAUTHN_RP_ID` | the issuer's host | WebAuthn relying party ID. Every passkey is bound to it, so changing it later orphans them all |
| `WEBAUTHN_ORIGINS` | the issuer's origin | Comma-separated origins allowed in a passkey answer. Each must be `https` (http only in development), a bare scheme://host, on the RP ID's domain; anything else refuses boot |
| `HIBP_ENABLED` | `true` | Breached-password screening for new passwords through the k-anonymity range API |

Sign-up serves only when `SIGNUP_ENABLED` is true, the key is long enough and SMTP is configured;
the boot log names which condition is missing. Passkey sign-in, passkey management, consent, the
legal pages and GDPR export and deletion work whether or not sign-up is open.

## Social sign-in

| Variable | Default | What it decides |
|---|---|---|
| `DISCORD_CLIENT_ID` | unset | The Discord application's client ID |
| `DISCORD_CLIENT_SECRET` | unset | The application's client secret. One without the other refuses boot; both empty turns Discord off everywhere |
| `GITHUB_CLIENT_ID` | unset | The GitHub OAuth application's client ID |
| `GITHUB_CLIENT_SECRET` | unset | The application's client secret. One without the other refuses boot; both empty turns GitHub off everywhere |

The redirect URI to register in the Discord Developer Portal (OAuth2) is
`<REGISTRE_ISSUER>/api/login/social/discord/callback`. The scopes requested are `identify email`.
A client shows the button only when its seed entry carries `login_providers: [discord]`; the
account page lists the provider only when at least one enabled client does. A Discord email the
provider has not verified neither matches nor creates an account, and a sign-in that would create
one is refused while public sign-up is closed.

The redirect URI to register in GitHub OAuth settings is
`<REGISTRE_ISSUER>/api/login/social/github/callback`. The scopes requested are `read:user user:email`.
A client shows the button only when its seed entry carries `login_providers: [github]`; the
account page lists the provider only when at least one enabled client does. A GitHub email the
provider has not verified neither matches nor creates an account, and a sign-in that would create
one is refused while public sign-up is closed.

The legal texts live in `apps/api/modules/legal/content/<document>.<lang>.md` and their versions in
`apps/api/internal/consent`. Changing a version is how everyone is asked to accept again at their
next sign-in. The texts are drafts; `LEGAL_TODO.md` lists what a lawyer must fill in.

## Deletion notices

Each application may declare a `deletion_uri` (seed field, or the dashboard's client page). When
an account is erased, Registre pushes a signed `user.deleted` Security Event Token to it
(`Content-Type: application/secevent+jwt`, RS256, `typ: secevent+jwt`); SPEC §6 lists what the
receiver verifies. A notice counts as delivered only on a 2xx from that URI, and a failure is
retried with backoff. The URI must not equal `directory_sync_uri`: a sync handler accepts any POST
with 202 and only schedules a pull, so it would look delivered and erase nothing; the seed and the
dashboard refuse it. Whatever the push does, `GET /api/directory/deletions` (HTTP Basic as the
application client) lists the app's erasures for sixty days and is the source of truth.
