# registre — Specification

The Facile Suite's own OIDC provider, replacing Authentik.

Read this before writing any code. It carries the decisions, the contract, and the reasoning
behind both. Where this file disagrees with the code, the code is wrong until this file is
amended in the same commit.

Companion document: [`porte/SPEC.md`](https://github.com/FacileStudio/porte/blob/main/SPEC.md).
`porte` is the client half and its contract is **frozen**. registre exists to satisfy it.

---

## 1. What registre is

| Layer | What | Who owns it |
|---|---|---|
| Protocol | OIDC — discovery, JWKS, auth code flow | Nobody here. `zitadel/oidc` does it |
| App plumbing | `/auth/oidc` routes, sessions, cookie, user upsert, `SSO_ONLY`, role hook | `porte`, unchanged |
| **Identity provider** | **The server that issues identities** | **registre**, at `sso.facile.studio` |

registre takes exactly one row of that table — the one Authentik occupies today. The other two
do not move. `porte` speaks standard OIDC and does not learn that its issuer changed.

### What it is not

- **Not a replacement for `porte`.** `porte` keeps its name and its job. `SPEC.md:31` of that
  repo is explicit: a library names the component it contributes *inside* the app — like
  `tronc`, `caisse`, `enveloppe` — "never an external service". porte stops being Authentik's
  client and becomes registre's, with zero lines changed in eight apps.
- **Not a directory service.** No LDAP, no SCIM, no SAML. Nothing in the suite speaks them.
- **Not multi-tenant.** One organisation. If that changes it is a rewrite, and that is fine.
- **Not a package registry.** The name is a hazard: "registry" is loaded in tech, and `Spore`
  already ships as the suite's package manager. Every description of this repo says *identity
  provider* and never the word registry.

---

## 2. Why it exists — the evidence

Not a capability gap. Authentik can do what the suite needs. The problem is how it is operated:

> `authentik-config/README.md`: "The running copies live inside Authentik, so **this repo is a
> mirror, not the source. Read the live object before trusting a file here.**" … "The two
> policies are **pasted into** Customisation → Policies … attached to a provider's property
> mappings, **one provider at a time**."

The identity layer is the only component in the suite configured by clicking, and its
version-controlled copy disclaims its own accuracy. A question as ordinary as "can we issue a
service-account token with an audience our apps validate?" is unanswerable without opening a web
UI and reading objects one provider at a time.

Two smaller costs, both real:

- Authentik's built-in `email` scope mapping hardcodes `email_verified: false`, so email-based
  account adoption can never fire. Recorded as a scar; unfixable from outside.
- The identity layer is the one Python component in a Go and TypeScript suite.

**This overrides a gate this project set for itself.** `porte/SPEC.md:112` says reopen the
question only when porte runs on all apps *and* the OIDC contract has been stable for months
*and* something concrete is blocked. As of 2026-08-23 that is **one of three**: porte has 8+
adopters, the contract is weeks old, and nothing is confirmed blocked. Proceeding is a
deliberate decision, recorded here so nobody later reads it as an oversight. It is also why §5
starts with the half that does not touch human authentication.

---

## 3. Decisions already taken

| Decision | Why |
|---|---|
| **Name: `registre`** | `porte/SPEC.md:111` pre-committed only to a register — institutions that issue identity papers. `consulat` read too formal. `guichet` collides with [Deuxfleurs' guichet](https://git.deuxfleurs.fr/Deuxfleurs/guichet), an LDAP console in the same domain. `portail` is already the org's static employee portal *and* fights with `porte` — door and gate are the same object |
| **Repo `Registre`, capitalised** | Case encodes kind in this org: every one of the 18 **app** repos is capitalised, every **lib** is lowercase. registre is an app — Postgres, `apps/api` + `apps/client`, deployed on la ruche. Only the repo and module path carry the capital; hostname, service, database and packages stay lowercase |
| **Private repo** | Matches `Casier`, the closest analogue for sensitivity and the only other private app. Not security through obscurity — an unfinished IdP simply does not benefit from an audience. Reversible later; publishing is not |
| **`zitadel/oidc`, not hand-rolled** | The attack-prone parts — PKCE verification, code binding, client authentication, token issuance — live in audited library code. Hand-rolling is defensible for `client_credentials` alone and indefensible once a browser flow exists |
| **Three config surfaces, not one** | YAML **seed** so a fresh suite instance bootstraps from files (the thing §2 is about), Postgres as runtime truth, admin UI for day-to-day. The seed is idempotent and applied at boot; it is not a phase-4 nicety |
| **Fresh identities, not seeded from Authentik UUIDs** | The alternative preserves `sub` and makes cutover a Traefik change, fully reversible. Rejected deliberately: carrying Authentik's UUIDs forever carries its legacy in the data. At this scale — three humans plus service accounts, every database under our control — the remap is a handful of statements. The price is that cutover is **not reversible**, which is why §9 is mostly verification |
| **Authentik stays up until phase 4 completes** | Both issuers can run at once. There is no moment where the suite has only an unproven IdP |
| **Phases in §5 are ordered by attack surface, not by value** | `client_credentials` has no interactive flow: no redirect_uri validation, no state/nonce, no session fixation, no CSRF, no recovery. It ships first because it is the half that cannot be got subtly wrong, and it happens to be the half that unblocks agent identities |

---

## 4. Scope

Full replacement, in five phases. Each ends somewhere usable.

### Phase 0 — skeleton and machine tokens — built 2026-08-23

Discovery, JWKS with rotation, `client_credentials`, service accounts, YAML seed. No login page.

**Exit, as written:** an agent authenticates to Courrier with a registre-signed token and porte
accepts it unchanged. A key rotation is exercised end to end without a single 401.

**The second half is done; the first half is not reachable and was wrong to write.** porte has no
path that accepts a signed token. `session/session.go:375` reads the `Authorization` header,
hashes what it finds and looks the hash up in the app's own `sessions` table — the credential is
opaque and revocable by construction, which `porte/session.go:9` says is deliberate and staying.
`porte/SPEC.md` does not contain the words `client_credentials` or *service account* anywhere. So
there is no version of "porte accepts it unchanged" for a machine token: accepting one is a
capability porte would have to gain, not a contract registre can satisfy. That is open question 6.

**Exit, corrected — what phase 0 actually proves:** a service account authenticates with
`client_credentials` and receives an RS256 JWT that verifies against the JWKS registre publishes,
carrying its own `sub`, the audiences the register grants it and nothing it asked for; and a key
rotation runs publish → activate → retire without invalidating a token issued before it. Both are
asserted end to end over real HTTP against real Postgres in `modules/oidc`.

### Phase 1 — human login

`authorization_code` + PKCE, exact-match `redirect_uri`, `state`/`nonce`, password login,
rate limiting and lockout. SvelteKit login page.

**Exit:** `porte`'s own test suite passes against registre — see §8.

### Phase 2 — integration parity

The `roles` claim, back-channel logout, `email_verified`. This is the phase that decides whether
apps actually work; §6 is its checklist.

**Exit:** OpenID Foundation conformance suite green. Deactivating a user drops them from every
app within seconds.

### Phase 3 — the hard half

TOTP, then WebAuthn. Account recovery over `facteur`. Admin UI. Break-glass (§7).

**Exit:** a human can enrol a second factor, lose it, and recover — without an administrator
touching the database.

### Phase 4 — migration and cutover

Import, remap, cut over one app, soak, then the rest. Decommission Authentik. See §9.

**Exit:** every app on `sso.facile.studio` backed by registre, zero orphaned subjects,
Authentik off.

---

## 5. Package layout

Go-family suite app, `GoSvelteBoilerplate` shape. Module `github.com/FacileStudio/Registre/apps/api`
— never bare `module api`.

**A package under `modules/` owns HTTP and says so in `router.go`. A domain
service that owns no routes lives under `internal/`.** That is the suite's rule,
it is what `filet.yml` enforces, and it is worth stating because the first draft
of this section got it wrong: `keys` and `clients` were listed under `modules/`,
where they arrived with no routes and an exception written into `filet.yml` to
excuse them. Suppressing an architecture rule to fit a layout is the erosion the
rule exists to prevent, so the packages moved instead. They never need to move
back: `admin/` below is the module that owns their HTTP when the dashboard
arrives in phase 3.

```
apps/api/
  main.go              func main() { os.Exit(run()) }; a failed migration exits 1
  migrations/          goose -s sequential, own package, //go:embed *.sql
  schemas/             GORM models
  modules/
    oidc/              the zitadel/oidc Storage implementation — the load-bearing file
    admin/             the dashboard's API (phase 3), over the services below
  internal/
    keys/              signing keys and rotation (§7)
    clients/           the register: applications, service accounts
    credentials/       argon2id, and the ceiling on how much of it runs at once (§7)
    users/             people (phase 1)
    seed/              idempotent YAML bootstrap
  cmd/import-authentik/  phase 4 only
apps/client/           SvelteKit 5 + muse admin UI and login page
```

---

## 6. The contract registre must honour

`porte`'s contract is frozen and registre is the party that has to satisfy it. Each line below is
a thing that breaks silently if missed.

| Requirement | Source | What breaks if missed |
|---|---|---|
| Standard discovery + JWKS | `porte/oidc/oidc.go:118` builds its verifier from provider discovery | porte will not start against the issuer |
| `authorization_code` + PKCE, enforced server-side | `porte/SPEC.md` §12: Authentik's token endpoint **enforces** PKCE, redirect URI and client auth "rather than trusting the client to have sent them" | A downgrade nobody notices |
| CLI loopback flow: `?flow=cli&port=N&cli_state=S` → one-time code → `POST /auth/oidc/exchange` | `porte/porte.go:91-94`, `oidc/handlers.go:105` | Every suite CLI's `login` stops working |
| **`roles` claim: flat array, filtered per client** | `porte/SPEC.md:109` — "already filtered and stripped for that app" | Authorization breaks in every app, silently and differently |
| **Back-channel logout** | `porte/SPEC.md:110` — "the *only* way a downstream app learns a session was terminated administratively"; porte consumes it at `/auth/backchannel-logout` | Deactivating a user stops propagating. A security capability regressing quietly |
| `email_verified` emitted **truthfully** | Authentik hardcodes `false`; porte gates its email fallback on it | Today: adoption never fires. Under registre this becomes a capability we gain |
| Issuer stays `sso.facile.studio` | `porte/SPEC.md:106` — the host was renamed off `porte.` precisely so it "survives replacing Authentik" | Every app needs an `OIDC_ISSUER` change instead of none |

Reuse `porte/local/password.go`'s argon2id parameters **verbatim** — 64 MiB, three passes, two
lanes — so password hashes stay portable in either direction.

---

## 7. Security — decide these before the code that depends on them

**Key rotation is the most likely cause of a suite-wide outage.** Publish a new key in JWKS
*before* signing with it; retain the retired public key for at least one maximum token lifetime
after last use. Build this in phase 0, with a test that rotates and asserts no request 401s.

**Break-glass, before any cutover.** If registre is the only door and its administrator account
is lost, all eleven apps are gone at once and there is no cheap recovery. Either a local
administrator that does not federate, or Authentik stays warm as a fallback issuer. Decide,
implement, and *test the recovery* — an untested break-glass is decoration. This gates phase 4.

**A refusal must cost what an acceptance costs.** No early return before the argon2 hash on the
login path; that is a timing oracle for account existence. `internal/credentials` folds the
equalisation into the one function that verifies, so no caller can forget it, and
`scripts/check.sh` refuses a second importer of `porte/local`.

**Cap how many hashes run at once, or the equalisation above is a denial of service.** argon2id at
porte's parameters allocates 64 MiB per call, and the token endpoint must hash before it can refuse
a bad credential — so without a ceiling every concurrent request is 64 MiB of registre's own memory,
from anyone, unauthenticated. Measured here before the ceiling existed: 148 MB resident for one bad
credential, 3.2 GB at fifty concurrent, 11.1 GB at two hundred. Keycloak shipped the same fix for
the same bug (keycloak#28477): a semaphore defaulting to the core count, because argon2 is CPU bound
and hashing more at once than there are cores buys nothing. Callers past the ceiling **wait**; a
refusal would reach the client as `invalid_client`, which is a lie about their credentials.

**No enumeration in recovery.** Same response and same timing whether the address is known.

**Never a credential in a URL.** `porte/session.go:310` reads only the cookie and the
`Authorization` header for exactly this reason: "a credential in a URL lands in access logs,
referrers and browser history."

---

## 8. Validation — the harness already exists

**Point `porte`'s test suite at registre.** porte was written as a strict relying party and its
tests run "behind a real JWKS", with a token endpoint that enforces PKCE, redirect URI and client
authentication. It will refuse anything sloppy. This is the cheapest and highest-signal
conformance check available and it costs nothing to run.

Then the **OpenID Foundation conformance suite** — the actual answer to "how do I know this is
correct", and the gate on phase 2.

Then **differential testing**: the same client and flow against Authentik and registre, comparing
the token and claims byte for byte. Divergence is a bug in registre until proven otherwise.

---

## 9. Migration — fresh identities, so verification is the safety net

Apps match accounts on **`(provider, subject)`**, and the storage differs by porte version:

- Newer apps carry a `porte_identities` table — `porte/pg/pg.go:208` matches on
  `claims.Provider, claims.Subject`.
- Older apps carry `users.oidc_subject` with a `uniqueIndex`.

**Audit all 8+ apps and record which shape each one has before writing a single `UPDATE`.**

The tooling in `cmd/import-authentik/` must provide:

1. Export from Authentik, mint fresh registre ids, emit an old-sub → new-sub map as an artifact.
2. `--dry-run` printing every change it would make, per app.
3. A verification pass asserting **zero orphans** (no app row whose subject is absent from
   registre) and **zero duplicates** (no subject claimed twice). Getting this wrong is the
   account-takeover class `porte` fixed in v0.2.3.
4. A restore-from-backup path that has been *executed*, not just written.

Cut over one app by changing `OIDC_ISSUER`, soak it, then the rest. Authentik goes off last.

---

## 10. Open questions — settle before coding the affected part

1. **Does the YAML seed cover human users?** Bootstrapping a fresh suite instance implies yes.
   If so it must seed *invite tokens*, never password hashes — a committed hash is a committed
   credential, and the seed file is in git.
2. **What happens to Authentik's two expression policies?** The avatar policy and the settings
   authorization policy in `authentik-config` are not part of the OIDC contract. Reimplement,
   or drop and accept the behaviour change. Decide before phase 3's admin UI.
3. **Does porte need a second verifier to run dual-issuer?** `porte/oidc/oidc.go:118` builds one
   verifier from `ClientID`. Running Authentik and registre side by side during phase 4 may
   require porte to hold two. Verify early — it is the difference between a staged cutover and a
   flag day.
4. **Session lifetime and refresh policy.** Authentik's current values are unrecorded. Read them
   off the live instance *before* it is decommissioned.
5. **Does registre emit to `antenne`?** Login, logout and administrative changes are exactly the
   events the bus exists for, keyed on `actor_email`. Emit-only, if so.
6. **How does an app consume a machine token?** Found while building phase 0, and it blocks the
   half of §4's exit that was struck through above. porte authenticates a bearer by hashing it and
   finding the row; a registre-signed JWT matches nothing. Two ways to close it, and the choice is
   porte's to make, in a version of its own:
   - **Verify offline.** porte grows a bearer path that, on a token shaped like a JWT, verifies it
     against the issuer's JWKS and checks `aud` against its own client id. No round trip, no new
     failure mode at request time, and it inherits the key rotation phase 0 already proves. It
     also means a revoked token keeps working until it expires, which is what the short default
     lifetime is for.
   - **Introspect.** porte calls `/oauth/introspect` with its own client credentials. Revocation
     is immediate and the app learns the scopes without parsing anything, at the cost of a round
     trip on every request and an identity provider that is now in the hot path of every app.
   Offline verification is the one to build; introspection already works and is there for the case
   that needs it. Whichever lands, the app-side identity is `(issuer, sub)` exactly as it is for a
   human — a service account is a principal, not a special case.

---

## 11. Repo conventions

Kind **app** under `DOCS-STANDARD.md`, so `README.md` needs Title, What it does, Stack, Quick
start, Configuration, Structure, Documentation and Footer, and `docs/` needs `README`,
`architecture`, `configuration`, `development` and `deployment`. Scaffold them in phase 0; at
phase 4 nobody wants to write deployment notes.

- `filet` gate clean; do not raise thresholds to pass.
- Godoc on exported symbols. **No inline comments inside function bodies** — hoist to the godoc
  above the declaration.
- One container, one Traefik router, one hostname. `expose`, not `ports`.
- `casier.yml` declaring project and environment, like every other app.
- Conventional commits; the shared hook adds the gitmoji itself.

---

## 12. Prior art, and what it actually says

`porte/SPEC.md:112` — "Rebuilding Authentik is **not** decided … Reopen the question only when
`porte` runs on all seven apps, the OIDC contract has been stable for months, and something
concrete is blocked by Authentik."

That gate is one of three (§2). It was written by people who were right to write it, and the
honest reading is that this project is starting early on purpose. The mitigations are that
phase 0 does not touch human authentication, Authentik stays up until phase 4, and the
validation in §8 is somebody else's test suite rather than our own optimism.
