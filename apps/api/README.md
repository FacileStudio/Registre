# registre — API

The OpenID Provider itself. Go 1.26, Chi, GORM, PostgreSQL 16, on
[tronc](https://github.com/FacileStudio/tronc), with the protocol handled by
[`zitadel/oidc`](https://github.com/zitadel/oidc).

```sh
cp .env.example .env      # fill in ENCRYPTION_KEY: openssl rand -base64 32
mise run dev
```

## Layout

| Path | Owns |
|---|---|
| `modules/oidc` | The `op.Storage` implementation: who gets a token, for how long, naming whom |
| `internal/keys` | Signing keys, the JWKS, and the rotation lifecycle |
| `internal/clients` | The register: reading a client, verifying its secret, writing a declaration |
| `internal/credentials` | argon2id, and the ceiling on how many hashes run at once |
| `internal/seed` | The declarative YAML bootstrap, applied at every boot |
| `migrations/` | The schema. The models in `schemas/` only describe what a migration built |

A package under `modules/` owns HTTP and declares it in `router.go`. A domain
service that owns no routes lives under `internal/`. `filet.yml` enforces it
with no exceptions.

## Tests

```sh
sh ../../scripts/check.sh
```

The key rotation and token issuance tests need a real PostgreSQL and skip
loudly without `TEST_DATABASE_URL`. See
[docs/development.md](../../docs/development.md).

Two things `scripts/check.sh` refuses that a compiler will not: `AutoMigrate(`
outside test code, because `migrations/` owns the schema; and an import of
`porte/local` outside `internal/credentials`, because argon2 has to stay behind
the concurrency ceiling.
