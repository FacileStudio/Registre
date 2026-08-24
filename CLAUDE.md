# Registre

The Facile Suite's own OIDC provider. Go + SvelteKit, deployed at `sso.facile.studio`.

**Read [SPEC.md](SPEC.md) first, every time.** It is the source of truth for what registre must
do and why. Where the code disagrees with it, the code is wrong until SPEC is amended in the same
commit.

## Where it is

**Phase 0 is built** (2026-08-23): discovery, JWKS with rotation, `client_credentials`, service
accounts and the YAML seed. `sh scripts/check.sh` is the gate; the tests that matter need a real
Postgres and skip loudly without `TEST_DATABASE_URL`.

**[ROADMAP.md](ROADMAP.md) is where to start.** It carries what is left, what is blocked on a
human, what has never been verified, and what was deferred on purpose with the reason. Read it
before proposing work, or you will propose something already decided against.

**No app can consume a registre token yet.** porte authenticates a bearer by hashing it and
finding the row in its own `sessions` table; a signed JWT matches nothing. That is SPEC §10
question 6 and it is a porte change, not a registre one. Do not "fix" it here.

There is no `apps/client`. That is deliberate: phase 0 has no login page and no admin UI, so a
scaffolded client directory would be a promise the repository does not keep. Add it with the login
page in phase 1, not before.

`modules/oidc` is the only module, and that is the rule rather than the current state: a package
under `modules/` owns HTTP and declares it in `router.go`; a domain service that owns no routes
lives under `internal/`. `filet.yml` enforces it and carries **no** architecture exceptions.
`keys` and `clients` were briefly under `modules/` with an exception written in to excuse them,
which is the erosion filet exists to prevent. Do not add one back.

## The three things most likely to be got wrong

1. **The contract is porte's, and it is frozen.** SPEC §6 lists what registre must emit. The
   `roles` claim filtered per client and back-channel logout are the two that break apps
   *silently* if missed — nothing errors, authorization just quietly stops working.
2. **Key rotation.** Publish a new key in JWKS before signing with it; keep the retired public
   key for a full token lifetime after last use. Getting this wrong 401s all eleven apps at once.
3. **Cutover is not reversible.** Fresh identities were chosen deliberately (SPEC §3), so the
   migration's verification pass — zero orphans, zero duplicate subjects — is the only safety
   net. SPEC §9.
4. **argon2 is 64 MiB a call, on a public endpoint.** The token endpoint must hash before it can
   refuse a bad credential, or an unknown client answers faster than a real one and that is an
   account-existence oracle. Which means every concurrent request is 64 MiB of registre's memory,
   from anyone. `internal/credentials` holds the one process-wide ceiling and folds the timing
   equalisation in so no caller can forget it; `scripts/check.sh` refuses a second importer of
   `porte/local`. Measured before the ceiling: 3.2 GB at fifty concurrent bad credentials, 11.1 GB
   at two hundred. Do not route around it.
5. **The register writes only the columns a declaration owns.** `clients.Upsert` names them
   explicitly, and the default is to leave a column alone, because the opposite default already
   cleared `disabled_at` — a revoked service account came back live on the next boot with its old
   secret, having been reported as revoked.

## Conventions

- Kind **app**: `apps/api` + `apps/client`, module `github.com/FacileStudio/Registre/apps/api`,
  never bare `module api`.
- Migrations: goose `-s` sequential, own package, `//go:embed *.sql`. A failed migration exits 1
  via `func main() { os.Exit(run()) }`.
- `filet` clean; do not raise thresholds to pass.
- Godoc on exported symbols. **No inline comments inside function bodies** — hoist them into the
  godoc above the declaration.
- muse tokens, never hex. Svelte 5 runes, no legacy stores.
- Conventional commits; the shared hook adds the gitmoji itself, so do not type one.

## Do not

- Rename or replace `porte`. It keeps its name and becomes registre's client, unchanged.
- Hand-roll the OIDC protocol. `zitadel/oidc` owns PKCE, code binding, client auth and token
  issuance for a reason.
- Call this a registry anywhere in prose. It is an identity provider.
