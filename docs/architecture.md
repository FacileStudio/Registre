# Architecture

registre is one Go binary and one PostgreSQL database. It answers as an OpenID Provider at
`sso.facile.studio` and it holds three things: who may ask for a token, the keys tokens are signed
with, and a record of every token issued.

## What owns what

| Package | Owns |
|---|---|
| `modules/oidc` | The `op.Storage` implementation. Who gets a token, for how long, naming whom |
| `internal/keys` | Signing keys: minting, sealing, the JWKS, and the rotation lifecycle |
| `internal/clients` | The register. Reading a client, verifying its secret, writing a declaration |
| `internal/credentials` | argon2id, and the ceiling on how many hashes run at once |
| `internal/seed` | The declarative bootstrap, applied at every boot |
| `internal/crypto` | AES-256-GCM, used for one thing: sealing private keys at rest |
| `migrations/` | The schema. Not the GORM models, which only describe what a migration built |

A package under `modules/` owns HTTP and declares it in `router.go`. A domain service that owns no
routes lives under `internal/`, which is why `keys` and `clients` are there and why `filet.yml`
carries no architecture exceptions.

The protocol itself is `zitadel/oidc`. PKCE verification, authorization code binding, client
authentication and token issuance live in audited library code because those are the parts that
are attack-prone and boring to get right. `modules/oidc` is the half that is specific to this
suite, and it is where every security decision registre makes actually lives.

## The token path

A service account posts to `/oauth/token` with its client id and secret in a basic auth header:

1. `zitadel/oidc` parses the request and calls `Storage.ClientCredentials`.
2. `internal/clients` looks the row up and hands the secret and the stored hash to
   `internal/credentials`, which verifies with argon2id at porte's parameters. A missing client, a
   disabled one and a wrong secret all arrive holding an empty hash, and an empty hash costs what a
   real one costs, so an unknown id takes the same sixty milliseconds a known one does. The
   equalisation lives inside the verifier rather than at the call site because a call site can
   forget it and the omission is invisible in review.

   The verifier also holds the process-wide ceiling on how many of those 64 MiB hashes run at
   once. Without it the endpoint is a memory amplifier: one unauthenticated request buys 64 MiB of
   the server. See [configuration](configuration.md#the-argon2-ceiling).
3. `Storage.ClientCredentialsTokenRequest` decides what is granted. A scope the register does not
   hold is refused rather than dropped, and the audience comes from the row rather than from the
   request: an agent cannot name the apps its own credential works at.
4. `Storage.CreateAccessToken` writes an `access_tokens` row and returns its id, which becomes the
   token's `jti`.
5. The library signs an RS256 JWT with the active key and returns it.

The app on the other end verifies that JWT against the JWKS, offline. The row in `access_tokens`
is not the credential; it exists so a token can be revoked before it expires, and so the question
"which agent called this app on Tuesday" has an answer. Revocation matches on the client id as
well as the token id, because RFC 7009 §2.1 requires it and because without it any client holding
valid credentials could revoke any other client's token.

## The key lifecycle

One key signs at a time. The rotation runs on a ticker and every step is idempotent, so a missed
tick costs nothing but time:

```
                    publish delay                     retention
                  ├───────────────┤              ├────────────────────┤
  ── active ──────┬───────────────┬── retired ───┬──────────────────── deleted
                  │               │              │
             published        activated       replaced
             (in the JWKS,   (starts signing) (still in the JWKS)
              signs nothing)
```

Both ends are the same outage seen from opposite sides. A key that signs before it is published
401s every relying party whose key set is cached. A key dropped while tokens it signed are still
valid does the same thing later, and is harder to attribute. The publish delay must outlast the
slowest relying party's key set cache; the retention must outlast one token lifetime, and
`env.Load` refuses to start if it does not.

Two replicas rotating at the same second would each mint a key and each try to promote it, so
`Rotate` takes a transaction-scoped Postgres advisory lock, and a partial unique index on
`state = 'active'` is the backstop if the lock is ever wrong.

Private keys are sealed with AES-256-GCM under `ENCRYPTION_KEY` and never leave the process in the
clear. Public halves are stored unsealed so the JWKS can be served without touching the root key.

## What phase 0 does not have

No login page, no human users, no password identities, no `roles` claim, no back-channel logout,
no refresh tokens. The `op.Storage` methods behind all of those exist and refuse, naming the grant
they are not, rather than returning something plausible. SPEC §4 orders the phases by attack
surface, and `client_credentials` came first because it has no interactive flow to get subtly
wrong.

`zitadel/oidc` writes `authorization_code`, `implicit` and the JWT bearer grant into the discovery
document whether or not their storage exists. Every endpoint it advertises is mounted, so probing
one gets a clean OIDC error rather than a 404 from whatever else claims the path.

## Two things about the library that are not obvious

`op.WithLogger` is a no-op: it accepts a `*slog.Logger` and discards it. The library logs through
the package-level `slog`, so `main` calls `slog.SetDefault` and that is the only reason the
provider's refusals share tronc's format, honour `LOG_LEVEL`, and reach Journal.

Its router registers `/healthz` and `/ready` of its own, so mounting the provider at the root would
shadow the `/ready` the container's health check calls. `Mount` registers each path by name instead,
and `Paths` is that list.
