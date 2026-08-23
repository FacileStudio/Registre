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
| `REGISTRE_KEY_ROTATION_INTERVAL` | `720h` | How long a key signs before it is replaced |
| `REGISTRE_KEY_PUBLISH_DELAY` | `15m` | How long a new key sits in the JWKS before it signs |
| `REGISTRE_KEY_RETENTION` | `2 ×` the token lifetime | How long a retired public key stays in the JWKS |

Two relationships are enforced at boot rather than discovered in production:

- **`REGISTRE_KEY_RETENTION` must be at least `REGISTRE_ACCESS_TOKEN_TTL`.** Below it, a retired
  key leaves the JWKS while tokens it signed are still valid, and every app holding one 401s.
- **`REGISTRE_KEY_PUBLISH_DELAY` must be shorter than `REGISTRE_KEY_ROTATION_INTERVAL`.** Above
  it, the next key is published before the current one has begun signing and the state machine
  never settles.

The publish delay has to outlast the key set cache of the slowest relying party in the suite.
`go-oidc`, which every app uses through porte, refetches on an unknown `kid` and so recovers on
its own; fifteen minutes is the margin for anything that does not.

`docker-compose.yml` leaves `REGISTRE_KEY_RETENTION` empty on purpose. Pinning a literal there
would break the derived default silently the moment somebody raised the token lifetime alone: the
boot guard would refuse to start, correctly, and the reason would be a value nobody set.

## The argon2 ceiling

| Variable | Default | What it decides |
|---|---|---|
| `REGISTRE_VERIFY_CONCURRENCY` | one per core | How many argon2 verifications run at once |

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

Both or neither: with one missing, nothing ships and nothing says so.

**This is the suite's auth audit trail, so treat it as required in production.** registre's own lines
are the key lifecycle and the seed; the ones that matter more come from `zitadel/oidc`, which logs
every refusal it issues — an unknown client, a scope the register does not grant, an unsupported
grant. Those reach Journal only because `main` calls `slog.SetDefault`: `op.WithLogger` accepts a
logger and discards it, so the library emits through the package-level slog and nothing else can
intercept it. Remove that call and the provider's refusals silently revert to slog's own text
format, at slog's own level, outside Journal.

Emitting to `antenne` is a separate question and still open — SPEC §10 question 5.
