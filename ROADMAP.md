# Registre Roadmap

> The Facile Suite's own OIDC provider, replacing Authentik at `sso.facile.studio`.
>
> [SPEC.md](SPEC.md) owns the decisions, the frozen contract and the phase definitions. This file
> tracks only what is done and what is left. Where they disagree, SPEC wins.
>
> Last updated 2026-08-24: phase 3 closed except WebAuthn. TOTP with recovery codes, an account
> session of registre's own, password recovery over facteur, and a tested break-glass. The client
> is rebuilt on muse with an app shell, and a bug that made every deep link unreachable is fixed.
> Phase 4 is blocked on reading the live Authentik instance rather than on code.

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
      when phase 0's first real consumer exists. ~~Until then the database is the only source of~~
      **Done 2026-08-24**: `seed.yaml` committed, mounted read-only, applied at boot — the
      `suite-ci` account exists in production and its secret lives in Dokploy's compose
      environment (copy in `~/registre-secrets/registre.env` on lucy). Applications get added at
      cutover.
- [ ] Back up `ENCRYPTION_KEY` somewhere that is not the Dokploy panel. It cannot be regenerated;
      see [docs/deployment.md](docs/deployment.md).

## Next, in order

1. [x] **Point porte at registre.** Done 2026-08-23: porte commit `e857531` adds
       `oidc/registre_test.go`, `TestNewBootsAgainstRegistre`. Skips unless `REGISTRE_ISSUER_URL`
       is set; run
       `REGISTRE_ISSUER_URL=https://sso.facile.studio go test ./oidc -run Registre`.
       Builds the full kit (`session.New` + `oidc.New`) against a live registre and asserts the
       discovery document parses and carries a `jwks_uri`. Passed against the deployed instance.
       It proves discovery only; verifying an actual minted JWT still needs real client
       credentials or a published local port.
2. [x] **Extend the discovery test to a full token round trip, decide SPEC §10 question 6, build
       it in porte.** Done 2026-08-24, three slices: `modules/oidc/roundtrip_test.go` mints a real
       `client_credentials` JWT over HTTP and verifies it offline against `/keys` (offline test
       green; the live variant skips until a service account exists on sso.facile.studio);
       SPEC §10 q6 answered (offline JWKS default); porte grew `oidc/jwt` — a kid-cached verifier
       wired behind `OIDC_MACHINE_AUDIENCE`, opaque tokens unchanged.
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

- [x] `authorization_code` + PKCE, exact-match `redirect_uri`, `state` and `nonce` — shipped
      2026-08-23, slice one
- [x] Password identities, argon2id at porte's parameters, through `internal/credentials` so they
      sit under the same ceiling
- [x] Rate limiting. The limiter lives in **`tronc/middleware`** as decided — `tronc v0.15.0`
      ships `middleware.Ratelimit`, and registre's login endpoint mounts it at 5 req/s, burst
      10, per source address. Lockout-by-account was deliberately not built: an attacker who
      can lock an account can lock a person out, which is the denial of service they wanted
- [x] `apps/client`: SvelteKit 5 + muse login page — shipped with slice one; the Go form stays
      behind it in production as the no-JS fallback
- [x] Refresh tokens, and only then advertise `offline_access` in discovery — shipped 2026-08-24.
      The grant rotates on every redemption (a replayed token kills the copy a thief holds), is
      stored only as a SHA-256 digest, and `REGISTRE_REFRESH_TOKEN_TTL` defaults to 30 days and
      must exceed the access token TTL or boot fails
- [x] Answer SPEC §10 question 1: yes, invite tokens never hashes; implementation lands with
      phase 3's admin slice (SPEC §10 amended in the same commit)
- [x] **Exit:** porte's own suite passes against registre — closed 2026-08-24. Both live gates
      ran green against `sso.facile.studio`: porte boots on the deployed discovery document
      (`TestNewBootsAgainstRegistre`), and a real `client_credentials` token minted by the
      seeded `suite-ci` account verifies through porte's offline verifier and is refused when
      tampered (`registre`'s TestALiveRegistreIssuesAVerifiableToken, and the consumer-side
      `TestAVerifierConsumesALiveRegistreToken` in porte/oidc/jwt). Porte's remaining tests are
      mock-based by design; the protocol-level conformance run is phase 2's gate, not this one.

## Phase 2 — integration parity

- [x] The `roles` claim, flat and filtered per client — shipped 2026-08-24. The `roles` scope is
      advertised now that the claim exists; visibility is opt-in per client (`visible_roles` on
      the register row, empty means none), because a role learned by an app that never needed it
      is a permission leak nobody notices. The claim rides in the ID token, where porte reads it
- [x] Back-channel logout — shipped 2026-08-24. Provider sessions are recorded per login, each
      app reached is appended to the session row, and ending them (end_session, or
      `registre user-disable`) POSTs an RFC-shaped logout token — signed RS256, `events` claim,
      no nonce — to every registered `backchannel_logout_uri`. Delivery is best-effort and
      logged; the session ends before notification, not after
- [x] `email_verified` emitted truthfully — shipped 2026-08-24. Operator-minted accounts are
      verified by construction (the operator attests the address by creating it); porte's email
      fallback can finally fire. Self-service signup will create unverified accounts
- [ ] **Exit:** OpenID Foundation conformance suite green; deactivating a user drops them from
      every app within seconds — the mechanism is built and tested (`user-disable`, back-channel
      delivery, session termination). The drill now exists as
      `TestDisablingAUserDropsThemFromEveryAppWithinSeconds` (modules/oidc/drill_test.go) and
      passed locally on 2026-08-24 with the logout token arriving 4 ms after user-disable; the
      run against sso.facile.studio remains. The OpenID Foundation suite needs a docker daemon,
      which lucy does not have — run it from a machine that does, pointed at a local registre.
      Discovery no longer over-advertises: `filteredDiscovery` strips implicit and jwt-bearer
      from `grant_types_supported`, asserted by TestDiscoveryAdvertisesOnlyServedGrants.

## Phase 3 — the hard half

- [x] `modules/admin`: the dashboard's API — shipped 2026-08-24. Operator-only over HTTP Basic
      checked through `identity.Verify` under the argon2 ceiling; clients (create, rotate secret),
      users (create, disable), invite tokens (issue, list, redeem at `/api/invites/redeem`,
      rate-limited like login), key status. SPEC §10 q1's invite-token seed discipline landed
      with it: single-use, expiring, shown once.
- [x] `apps/client` admin UI — shipped 2026-08-24. `/admin` (operator sign-in, users,
      invitations, clients, signing keys) and the public `/invites/redeem` password-set page;
      SvelteKit 5 + muse, secrets shown once with a copy-now warning. The dashboard authenticates
      with HTTP Basic held in browser memory; when a cookie session ever replaces it,
      X-Facile-CSRF comes with the change.
- [x] TOTP — shipped 2026-08-24. `internal/totp` over `pquerna/otp`, seed sealed under the root
      key like a signing key, RFC 6238 §5.2 replay guard on a `totp_last_step` column, ten
      single-use recovery codes stored as digests. Enrolment is a `Begin`/`Confirm` pair so an
      abandoned attempt can never gate a login. The second factor interposes at
      `POST /api/login/totp` through `internal/challenges`, which holds the "password was right"
      state server-side rather than handing it to the browser, with a five-attempt budget per
      challenge
- [ ] WebAuthn. Deferred with a reason: it needs a fixed origin to bind to and a credential store
      of its own, and TOTP plus recovery codes already closes the phase 3 exit. Doing both at once
      would have shipped neither well
- [x] Account recovery over `facteur`, with no enumeration and equal timing — shipped 2026-08-24.
      `internal/recovery` pays the argon2 cost on both paths and `modules/reset` answers the same
      status and the same body for a known and an unknown address, including on its own internal
      errors. Delivery is handed to a goroutine before the SMTP round trip starts, because a
      synchronous send is only on the known path and would put back the timing oracle the
      equalisation removes
- [x] `modules/admin`: the dashboard's API, which is what will own HTTP for `internal/keys` and
      `internal/clients` rather than moving them back under `modules/`
- [x] Break-glass, tested — shipped 2026-08-24. `registre break-glass --email <address>`, over
      `internal/breakglass`, in one transaction, with the drill in `breakglass_test.go`. SPEC §7
      carries the decision and why the fallback-issuer option was never available to an issuer
- [x] **Exit: closed 2026-08-24.** A human enrols a second factor at `/account/security`, and if
      they lose the device they spend a recovery code at the login challenge, or reset their
      password from `/recover` if that is what went. No operator and no database access is
      involved in any of it. Asserted end to end against a running binary over real HTTP, and in
      `internal/totp`, `modules/account` and `modules/login`

## Phase 4 — migration and cutover

**Blocked on reading the live Authentik instance, not on writing code.** `authentik-config`
carries only the two expression policies and one scope mapping; it holds no user or application
data, and its README says the repo is a mirror rather than the source. There is **no API token on
that instance**, so an export means a base64-encoded Django script over
`ssh ruche "docker exec ... ak shell -c"`. Nothing here should be written against a guess at what
comes back: an importer built blind, against a cutover SPEC §9 says is one-way, is a liability
rather than progress. Do the export first, by hand, and write the tool against the artifact.

Two facts already gathered that the tool has to satisfy:

- Every adopted app now carries `porte_identities` keyed `(provider, subject)`, and six of them
  (Agenda, Courrier, Nuage, Plume, Sablier, Vision) also still carry `users.oidc_subject` plus a
  one-shot backfill. `provider` is the **issuer URL**, so it changes per app at cutover, not just
  the subject.
- Authentik issues one issuer per application, `https://porte.facile.studio/application/o/<slug>/`,
  and registre serves a single issuer at the root. Every app therefore changes `OIDC_ISSUER`
  whatever else happens, which is what makes a one-app-at-a-time cutover possible.

- [ ] `import-authentik`: export, mint fresh ids, emit an old-sub to new-sub map. Note the
      repository's actual convention is a subcommand file beside `main.go`, as `user-create`,
      `user-disable` and `break-glass` are, not a `cmd/` directory
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

## Fixed on the way through, 2026-08-24

- [x] **Every client route except "/" was unreachable by URL.** `internal/web` rewrote an unknown
      path to `/index.html` and handed it to `http.FileServerFS`, which canonicalises that name by
      redirecting to `./`. So `/login` answered 301 to the origin root and the route was discarded:
      reload, bookmark and the redirect back from an OIDC flow all landed on the marketing page.
      Production was answering **404** for `/login` and `/admin` when this was found, so the login
      page had never worked at the deployed URL. The handler now writes the index bytes directly,
      `internal/web/web_test.go` asserts every route answers 200 and HTML, and an unrouted `/api`
      path answers JSON 404 instead of a page, which is the failure `env.go` already documents
      Journal losing reports to.
- [x] **The dashboard could only mint unusable applications.** `POST /api/admin/clients` accepted
      no `redirect_uris`, no `visible_roles` and no `backchannel_logout_uri`, and defaulted no
      grant types, so a client registered through the UI was refused at `/authorize` for a missing
      redirect URI and then for a missing response type. All three fields are accepted and shown
      now, the grants and scopes default to the seed's per kind so the two registration paths
      cannot disagree, and an application without a redirect URI is refused at creation with a
      message that names the problem.
- [x] **muse was pinned at `file:../../../muse`**, the only repo in the suite doing so, against a
      local checkout eleven commits behind `origin/main`. Now `github:FacileStudio/muse#v0.6.4`.
- [x] **`JOURNAL_BROWSER_URL` and `JOURNAL_BROWSER_KEY` were never forwarded by
      `docker-compose.yml`**, so the browser error reporting shipped in `c0ac7b4` could not be
      switched on in production however the Dokploy environment was set. Same trap the seed secret
      hit: compose forwards by name, and a variable this block does not list never reaches the
      container. Both are in the block now.
- [x] **`filet.yml` never linted the client.** It said TypeScript and Svelte would join the
      extension list "with the login page in phase 1"; phase 1 shipped long ago. `.ts` and
      `.svelte` are in the list now and `apps/client` is a required directory.

## Known small things

- [x] ~~Discovery over-advertises `authorization_code`, `implicit` and the JWT bearer grant~~ —
      fixed 2026-08-24, see the phase 2 exit note above
- [ ] `Ensure` and `Upsert` both read-then-create, so two replicas booting in the same instant
      means one fails on a unique violation and restarts into a correct state. Self-healing, noisy
- [ ] A retired key with a NULL `expires_at` stays in the JWKS forever. Only reachable by a manual
      edit, and the fail-safe direction, but it would mask the bug it guards against
- [ ] `filet test .` reports "no test suite detected" while `go test ./...` runs 34. That looks
      like filet's detector rather than this repo
- [ ] Filet findings that remain are forced: `//go:embed`'s package-level var, the four-return
      `CreateAccessAndRefreshTokens` (the `op.Storage` interface dictates the arity), three
      deliberate `_ = x.Close()` in cleanup paths, and `schemas.Client`'s 17 columns — one per
      register concern; splitting the model would scatter GORM tags for no reader benefit

## Open questions

SPEC §10 carries them in full. Short form, and none is answered:

1. Does the YAML seed cover human users? If so it seeds invite tokens, never hashes
2. What happens to Authentik's two expression policies — reimplement or drop
3. Does porte need a second verifier to run dual-issuer during phase 4
4. Session lifetime and refresh policy — read Authentik's values off the live instance **before**
   it is decommissioned
5. Does registre emit to antenne
6. ~~How does an app consume a machine token~~ — answered 2026-08-24, SPEC §10: offline JWKS
   verification is the default, introspection the opt-in
