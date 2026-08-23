# Registre Roadmap

> The Facile Suite's own OIDC provider, replacing Authentik at `sso.facile.studio`.
>
> [SPEC.md](SPEC.md) owns the decisions, the frozen contract and the phase definitions. This file
> tracks only what is done and what is left. Where they disagree, SPEC wins.
>
> Last updated 2026-08-23, the day phase 0 shipped.

## Read this first

**Nothing can use a registre token yet.** porte authenticates a bearer by hashing it and looking
the hash up in the app's own `sessions` table, so a signed JWT matches nothing. registre is a
provider with no consumer until that changes, and the change is in `porte`, not here. SPEC §10
question 6 has the two ways to close it.

**registre is deployed.** 2026-08-23: Dokploy compose on la ruche, `sso.facile.studio` behind
Traefik with a Let's Encrypt cert, `/ready` green, discovery and JWKS answering from the real
hostname. Secrets (`ENCRYPTION_KEY`, `POSTGRES_PASSWORD`) went into Dokploy's compose environment,
**not Casier**, because no machine this session had a working `casier` CLI. If Casier becomes the
suite's secret source of truth, migrate those two values there before anyone trusts `casier check`.

## Blocked on a human

- [ ] Mint a per-app `JOURNAL_TOKEN` from Journal's dashboard Keys page and set `JOURNAL_URL`
      (`http://journal-api:4010/api`) plus the token in Dokploy's environment for the compose. The
      SDK wiring is done and ships the moment both are set.
- [ ] Set `REGISTRE_SEED_FILE` with a mounted seed volume and one `*_SECRET` per service account,
      when phase 0's first real consumer exists. Until then the database is the only source of
      clients, which the boot log records.
- [ ] Back up `ENCRYPTION_KEY` somewhere that is not the Dokploy panel. It cannot be regenerated;
      see [docs/deployment.md](docs/deployment.md).

## Next, in order

1. [ ] **Point porte at registre.** The smallest useful cross-repo check available today: porte's
       `oidc.New` does discovery and builds its verifier, so a test asserting porte can boot
       against a running registre proves the discovery document is acceptable to the client that
       has to read it. Nothing outside this repo has ever looked at what registre serves.
2. [ ] **Decide SPEC §10 question 6 and build it in porte.** Offline JWKS verification is the one
       to build; introspection already works here if immediate revocation is wanted instead.
3. [ ] Phase 1.

## Phase 0 — machine tokens — shipped 2026-08-23

- [x] Discovery, JWKS, `client_credentials`, RS256 JWT access tokens
- [x] Service accounts with their own `sub`, scopes refused rather than narrowed, audience taken
      from the register and not from the request
- [x] Key rotation: publish, activate, retire, prune, under an advisory lock with a partial unique
      index as backstop
- [x] Boot guards for the two configuration relationships that cause a suite-wide 401, and for a
      changed `ENCRYPTION_KEY` (which otherwise looks healthy until the first token request)
- [x] Idempotent YAML seed that names its secrets rather than carrying them
- [x] Introspection and revocation, with the RFC 7009 and 7662 access checks
- [x] A process-wide ceiling on concurrent argon2, so the token endpoint is not a memory amplifier
- [x] `scripts/check.sh` gate, filet clean, CI, hooks installed and gating pre-push

Exit as written was wrong and is corrected in SPEC §4. What phase 0 actually proves is asserted
end to end in `apps/api/modules/oidc`.

## Phase 1 — human login

- [ ] `authorization_code` + PKCE, exact-match `redirect_uri`, `state` and `nonce`
- [ ] Password identities, argon2id at porte's parameters, through `internal/credentials` so they
      sit under the same ceiling
- [ ] Rate limiting and lockout. **The limiter belongs in `tronc/middleware`** — the suite already
      has eight hand-written copies and the ninth should not be written here
- [ ] `apps/client`: SvelteKit 5 + muse login page. `filet.yml` `requiredDirs` gains `apps/client`
      when it exists
- [ ] Refresh tokens, and only then advertise `offline_access` in discovery
- [ ] Answer SPEC §10 question 1: if the seed covers human users it must seed invite tokens, never
      password hashes
- [ ] **Exit:** porte's own test suite passes against registre

## Phase 2 — integration parity

- [ ] The `roles` claim, flat and filtered per client. Advertise the `roles` scope in discovery
      only once the claim behind it exists: a granted scope with no claim makes every role check
      in every app deny silently
- [ ] Back-channel logout. The only way a deactivation reaches an app holding its own session
- [ ] `email_verified` emitted truthfully, which is a capability the suite gains rather than keeps
- [ ] **Exit:** OpenID Foundation conformance suite green; deactivating a user drops them from
      every app within seconds

## Phase 3 — the hard half

- [ ] TOTP, then WebAuthn
- [ ] Account recovery over `facteur`, with no enumeration and equal timing
- [ ] `modules/admin`: the dashboard's API, which is what will own HTTP for `internal/keys` and
      `internal/clients` rather than moving them back under `modules/`
- [ ] Break-glass, tested. An untested break-glass is decoration, and it gates phase 4
- [ ] **Exit:** a human can enrol a second factor, lose it, and recover without anyone touching
      the database

## Phase 4 — migration and cutover

- [ ] `cmd/import-authentik`: export, mint fresh ids, emit an old-sub to new-sub map
- [ ] `--dry-run` printing every change per app
- [ ] Verification pass asserting zero orphans and zero duplicate subjects. Fresh identities were
      chosen deliberately, so this is the only safety net and cutover is one-way
- [ ] A restore-from-backup path that has been executed, not just written
- [ ] Audit which of the 8+ apps key on `porte_identities` and which on `users.oidc_subject`
      **before** writing a single `UPDATE`
- [ ] Cut over one app, soak, then the rest. Authentik off last
- [ ] **Exit:** every app on `sso.facile.studio` backed by registre, zero orphaned subjects,
      Authentik decommissioned

## Verification that has not happened

- [x] ~~porte has never been pointed at registre~~ — partially: the live discovery document is now
      served from `https://sso.facile.studio` and fetched successfully by hand; a porte test
      against it (Next item 1) is still open
- [x] Behind Traefik: the issuer is the real hostname, discovery fetched over https through
      Traefik + Cloudflare, 2026-08-23
- [ ] Rotation has only been driven by a fake clock. Nothing has watched a key rotate over real
      wall-clock time across a restart
- [ ] Multi-replica is untested. The advisory lock and partial unique index are correct by
      construction and the index is proven by rejecting a duplicate insert, but two processes have
      never actually raced
- [ ] OpenID Foundation conformance suite (phase 2 gate)
- [ ] Differential testing against Authentik: same client, same flow, compare claims byte for byte

## Deferred by decision, with the reason

- [ ] **Rate limiting.** The argon2 ceiling bounds memory absolutely; the residual is latency
      under flood, which an edge rate limit answers. `docker-compose.yml` carries a commented
      Traefik `ratelimit` middleware because the numbers are a deployment policy, not a default
- [ ] **`tronc/crypto`.** `internal/crypto` duplicates Casier's. Second app to need it, so it is
      the moment to propose the extraction rather than the third
- [ ] **antenne.** SPEC §10 question 5. Login, logout and administrative changes are exactly what
      the bus is for, emit-only, keyed on `actor_email`
- [ ] Ten `op.Storage` methods refuse by name rather than pretending. That is the phase ordering
      working, not a gap

## Known small things

- [ ] Discovery over-advertises `authorization_code`, `implicit` and the JWT bearer grant, which
      zitadel writes unconditionally. Every advertised endpoint is mounted and refuses cleanly, so
      it is an optimistic document rather than a 404
- [ ] `Ensure` and `Upsert` both read-then-create, so two replicas booting in the same instant
      means one fails on a unique violation and restarts into a correct state. Self-healing, noisy
- [ ] A retired key with a NULL `expires_at` stays in the JWKS forever. Only reachable by a manual
      edit, and the fail-safe direction, but it would mask the bug it guards against
- [ ] `filet test .` reports "no test suite detected" while `go test ./...` runs 34. That looks
      like filet's detector rather than this repo
- [ ] Five filet findings remain and all five are forced: `//go:embed`'s package-level var, the
      four-return `op.Storage` method, three deliberate `_ = x.Close()` in cleanup paths

## Open questions

SPEC §10 carries them in full. Short form, and none is answered:

1. Does the YAML seed cover human users? If so it seeds invite tokens, never hashes
2. What happens to Authentik's two expression policies — reimplement or drop
3. Does porte need a second verifier to run dual-issuer during phase 4
4. Session lifetime and refresh policy — read Authentik's values off the live instance **before**
   it is decommissioned
5. Does registre emit to antenne
6. How does an app consume a machine token (see Read this first)
