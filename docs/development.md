# Development

```sh
mise install          # go 1.25, lefthook
cd apps/api && cp .env.example .env
```

Fill in `ENCRYPTION_KEY` with `openssl rand -base64 32`, then:

```sh
mise run dev          # or: cd apps/api && go run .
```

The development defaults use a plain-http issuer on `127.0.0.1`, which registre accepts only
because `APP_ENV=development`. They also use a deliberately short key lifecycle so rotation can be
watched happening rather than reasoned about.

## A database

Postgres 16 is the only database, tests included. Either works:

```sh
docker run -d --rm --name registre-db -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_USER=postgres -e POSTGRES_DB=registre -p 127.0.0.1:5433:5432 postgres:16-alpine
```

```sh
initdb -D .local/postgres --no-locale --encoding=UTF8
pg_ctl -D .local/postgres -o "-p 5433" start && createdb -p 5433 registre
```

## The gate

```sh
sh scripts/check.sh          # gofmt, vet, test, the schema guards, then filet
sh scripts/check.sh --format # rewrite Go sources in place
```

It depends on nothing but a `go` on PATH. It is not run through `mise run` on purpose: `mise run`
resolves every tool in the merged config before running any task body, so one broken tool in a
global config would take the gate down with it.

Three guards in it are worth knowing about, none of which a compiler would catch:

- A non-Postgres driver in `go.mod` is refused. A test on SQLite builds a different schema from the
  struct tags and then passes, proving nothing about the DDL that ships, and it cannot run the
  partial unique index that enforces one active signing key.
- `AutoMigrate(` outside test code is refused, because `migrations/` owns the schema.
- An import of `porte/local` outside `internal/credentials` is refused. argon2id allocates 64 MiB
  per call, so every call site is a memory ceiling of its own; `internal/credentials` holds the one
  process-wide limit, and a second importer is a second uncapped path. See
  [configuration](configuration.md#the-argon2-ceiling).

## The tests that need a database

The key rotation and token issuance tests run a real provider over real HTTP against real
Postgres. Nothing in them is a mock: the point is that a token registre signs verifies against the
key set registre serves, and a fake of either half would only prove the fake agrees with itself.

```sh
docker run -d --rm --name registre-db \
  -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=registre_test \
  -p 127.0.0.1:5433:5432 postgres:16-alpine
export TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5433/registre_test?sslmode=disable'
```

Without it they skip, loudly, to stderr. Each test package gets its own Postgres schema, because
`go test ./...` runs packages concurrently and two of them truncating the same tables steal each
other's fixtures.

## Getting a token by hand

```sh
curl -s -u 'agent-claude:<the secret>' -d 'grant_type=client_credentials' \
  http://127.0.0.1:4000/oauth/token
```

The `access_token` is a JWT. Decode the middle segment to read `sub`, `aud`, `exp` and `client_id`;
verify it against `http://127.0.0.1:4000/keys`.
