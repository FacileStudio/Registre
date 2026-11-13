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
- **Not a package registry.** The name is a hazard: "registry" is loaded in tech. Every
  description of this repo says *identity provider* and never the word registry.

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
| **Public self-service sign-up, off until the owner opens it** | Decided 2026-11-04 for the public launch. `SIGNUP_ENABLED` defaults to false so a deploy never opens it. Email verification is required, passkeys are offered first, the password fallback follows NIST SP 800-63B-4 (15 to 128 characters, no composition rules, breached-password screening), and bots pay a self-hosted ALTCHA proof of work: no Google or Cloudflare service is involved |
| **Languages: fr, en, es, de** | The person's locale lives on their row and reaches apps as the standard OIDC `locale` claim. Every page a person meets (sign-in, sign-up, consent, legal, account, spaces, device approval) and every mail exists in all four; `?lang=xx` picks one. The operator dashboard and the plan section stay English for now |
| **Joining a space needs the person's consent** | Decided 2026-11-04. With public sign-up, "type an address, the account is in your space" became a way to push strangers into spaces. An owner or admin now sends an invitation; membership exists only after the person accepts |

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
    login/             the credential check and the second-factor step (phase 3)
    account/           the pages a human manages their own security from (phase 3)
    reset/             password recovery, request and redeem (phase 3)
  internal/
    keys/              signing keys and rotation (§7)
    clients/           the register: applications, service accounts
    credentials/       argon2id, and the ceiling on how much of it runs at once (§7)
    identity/          people (phase 1). SPEC's first draft called this users/
    seed/              idempotent YAML bootstrap
    invites/           single-use invitations (§10 q1)
    logout/            back-channel logout delivery
    web/               the embedded client build, served as the catch-all
    totp/              the second factor and its recovery codes (phase 3)
    challenges/        a password proven with a factor still owed (phase 3)
    accounts/          the session registre holds with a human directly (phase 3)
    recovery/          password reset over facteur (phase 3)
    breakglass/        the way back in when the way in is the problem (§7)
    billing/           the server-to-server client for Ardoise's billing API (§6)
apps/client/           SvelteKit 5 + muse admin UI, login, account and recovery pages
```

Two of those need a word, because neither was in the original layout.

**`internal/accounts` exists because registre cannot be its own relying party.** Every other app in
the suite gets its session from registre. registre has nowhere to get one, so the account pages
need a session of their own. It is a random bearer stored as a digest, hung off the provider
`sessions` row that login already creates rather than a table of its own, so `end_session`,
`registre user-disable` and back-channel logout all close the account pages in the same statement
that closes everything else. A second session table would have been a second thing to remember to
revoke.

**Subcommands live flat in `apps/api/`, not under `cmd/`.** `user-create`, `user-disable` and
`break-glass` are files beside `main.go` dispatched on `os.Args[1]`, which is the shape phase 1
already set. `cmd/import-authentik/` above is therefore aspirational rather than descriptive; when
phase 4 arrives it should match the convention the repository actually has.

---

## 6. The contract registre must honour

`porte`'s contract is frozen and registre is the party that has to satisfy it. Each line below is
a thing that breaks silently if missed.

| Requirement | Source | What breaks if missed |
|---|---|---|
| Standard discovery + JWKS | `porte/oidc/oidc.go:118` builds its verifier from provider discovery | porte will not start against the issuer |
| `authorization_code` + PKCE, enforced server-side | `porte/SPEC.md` §12: Authentik's token endpoint **enforces** PKCE, redirect URI and client auth "rather than trusting the client to have sent them" | A downgrade nobody notices |
| CLI loopback flow: `?flow=cli&port=N&cli_state=S` → one-time code → `POST /auth/oidc/exchange` | `porte/porte.go:91-94`, `oidc/handlers.go:105` | Every suite CLI's `login` stops working |
| **Device grant (RFC 8628) for a CLI whose browser is elsewhere** | The loopback flow assumes the browser runs on the machine that started the login | A login started over SSH sends the human's browser to *its own* `127.0.0.1`; the code expires unused and nothing says why |
| **`roles` claim: flat array, filtered per client** | `porte/SPEC.md:109` — "already filtered and stripped for that app" | Authorization breaks in every app, silently and differently |
| **Back-channel logout** | `porte/SPEC.md:110` — "the *only* way a downstream app learns a session was terminated administratively"; porte consumes it at `/auth/backchannel-logout` | Deactivating a user stops propagating. A security capability regressing quietly |
| `email_verified` emitted **truthfully** | Authentik hardcodes `false`; porte gates its email fallback on it | Today: adoption never fires. Under registre this becomes a capability we gain |
| Issuer stays `sso.facile.studio` | `porte/SPEC.md:106` — the host was renamed off `porte.` precisely so it "survives replacing Authentik" | Every app needs an `OIDC_ISSUER` change instead of none |

**The device grant is an addition to that contract, not a replacement.** The loopback row above
stays exactly as it is: a CLI on the same machine as the browser keeps using it and porte's
`?flow=cli&port=N&cli_state=S` is untouched. RFC 8628 covers the case loopback cannot — a login
started on a server, in a container or over SSH, where `127.0.0.1` on the browser's machine is
not the machine that is waiting. registre mints the pair of codes, the human approves the short
one on any device that has a browser, and the CLI polls the token endpoint with the long one.

`apps/client`'s `/device` is the human half, one route in the `(auth)` group. It takes the user
code typed or prefilled from `verification_uri_complete`, names the client and the scopes it
asked for, offers **approve and deny as equal-weight choices** because RFC 8628 makes
`access_denied` an answer the waiting CLI is told about rather than a cancelled dialog, and it
is the page a human opens on a phone — so the phone layout is the primary one, not the fallback.

**Delegation by token exchange (RFC 8693) is the other addition.** An app that holds a signed-in
human's token (Bulle, acting for them) trades it at the token endpoint for a token addressed to
exactly one other app, so that app never sees the first token and the human never pastes one. The
caller is a confidential client that authenticates normally and holds the grant; the register
(`clients.token_exchange`, declared per application in the seed) names the audiences it may ask for
and the scopes it may ask for there. The issued JWT carries `sub` = the human, `aud` = the one
audience **written verbatim** (no name resolution: it must equal the receiving app's
`OIDC_MACHINE_AUDIENCE`), `scope` = the granted scopes, `act.sub` = the caller and `client_id` = the
caller. It lives at most five minutes and never past the subject token, carries no refresh token
and no `roles`, and is recorded in `access_tokens` with `actor` set. The subject must be a live,
unrevoked access token that registre issued to the caller with the caller in its `aud`, for a user
who exists and is enabled; a delegated token is refused as a subject, so a delegation cannot be
delegated again. Every refusal of a class answers with the same body (`invalid_grant`,
`invalid_target`, `invalid_scope`), so the response does not say which check failed. Writes are
not granted yet: scopes are `<app>.read` or `<app>.write`, and only `.read` is seeded.

**Profile pictures ride the standard `picture` claim.** A human uploads one picture on
`/account`; registre crops it to a square, re-encodes it as PNG (which drops EXIF and anything
appended to the file) and serves it publicly at `<issuer>/avatars/<sub>`. That URL is stable per
human and always serves the current picture, with `Cache-Control: no-cache` and the upload id as
ETag, so every load revalidates and costs a 304 while nothing changed. `picture` is emitted in
the ID token and at userinfo under the `profile` scope only, and for every human whether or not
a picture exists: the URL answers 404 (cacheable for thirty seconds) until one is uploaded, so an
app that stored it at its first sign-in shows a first upload too, and renders initials meanwhile. The suite's apps store that URL at
sign-in and render it, so a new upload shows in every app on its next page load, with nothing
pushed and no re-sync. An app that copies the bytes instead (porte's `AvatarStore`) only sees a
change at the next sign-in, which is why no suite app should. Removing the picture makes the URL
answer 404 again.

**Who may use which app is registre's.** Each application has an access policy: `everyone`, the
default, admits every enabled user; `assigned` admits only users holding a grant to it. A grant
(`app_grants`) is one user, one app and the roles they hold there, so a person can be admin in
one app and nothing in another. The `roles` claim is the grant's roles intersected with the
client's `visible_roles`, still a flat array filtered per client as the table above requires.
The check runs wherever a human gets a token for an app: completing a login (password, second
factor or live session), approving a device code, redeeming a refresh token and exchanging a
token for another audience. Restricting an app or revoking a grant revokes the refresh tokens
that no longer qualify, so access ends within one access-token lifetime. Registre's own client
can never be restricted, so no operator can lock the dashboard. Every dashboard change is
written to `admin_events`.

**Shared spaces are registre's.** A space is a team, such as "Facile", that exists in every app
with the same members at the same rank, owner > admin > member (porte's `spaces` ladder). Humans
create spaces, invite mates (who accept) and change roles on `/spaces`; no app edits them. Apps pull
`GET /api/directory/snapshot`, authenticated with HTTP Basic as their own application client, and
make their tables match it. The snapshot is the whole state, so applying it is idempotent. After
every change registre bumps a version in the same transaction and POSTs an empty nudge to each
client's `directory_sync_uri`; apps also pull at boot and every five minutes, so a lost nudge
costs at most one interval. This is delivery at least once with an idempotent receiver, which is
what SCIM-style group provisioning needs, without SCIM's per-resource surface. Disabled humans
never appear in a snapshot, which is how disabling someone removes them everywhere. Each app hands
over the spaces it held before this existed exactly once (`POST /api/directory/import`): spaces
merge by name ignoring case, the highest role anyone held wins, unknown people are skipped, and a
second import from the same client is a no-op, so a restart cannot re-add someone an owner removed.
The shared client is `github.com/FacileStudio/porte/directory`, a nested module of the public porte
repository, because this repository is private and the apps' builds could not fetch it.

**Plan entitlements ride the snapshot.** Ardoise, the billing app, decides what each space and each
person may do and pushes it with `PUT /api/directory/entitlements`, authenticated with HTTP Basic as
its own application client, which must also be the client named by `ENTITLEMENTS_WRITER` (unset:
the route answers 404; another app: 403; bad credentials: 401). The body is
`{"revision", "spaces": [{"id", "entitlements"}], "people": [{"sub", "email", "entitlements"}]}`,
at most 1 MiB, unknown fields refused. Each entitlement key matches `^[a-z0-9_]+\.[a-z0-9_]+$`, a
scope holds at most 200, and each value is a JSON bool or an integer of at least -1, -1 meaning
unlimited; anything else is 400. The push is the whole truth: every scope it leaves out loses its
values. A push carries at most 10,000 scopes, written in batches. A revision older than the one
stored answers 409 `{"error":"stale revision","current":N}` and changes nothing; an equal one is
applied again so a retry converges; one more than 1,000,000 ahead is 400, so a single bad push
cannot make every later one stale. A space id registre does
not know is skipped and counted; a person resolves by subject, else by a verified email, and is
skipped otherwise. Applying bumps the directory version and writes the `admin_events` row in
the same transaction, then nudges the apps. The snapshot is read in one read-only REPEATABLE
READ transaction, so it never mixes two pushes. The snapshot then carries `entitlements_revision`, the last
revision applied (absent until the first push, and the only signal that ends an app's fail-open),
`entitlements` on each space, and a `people` list of `{"sub", "entitlements"}` for enabled humans
with personal values; the last two are omitted when empty, which apps read as "use the feature
default". Registre stores the values and
never interprets them.

**Owners manage the plan on registre's spaces pages.** `GET /api/spaces/{id}/billing` answers any
member the space's plan from Ardoise's `GET /billing/spaces/:id`, and an owner the published
catalog as well. `POST .../billing/checkout {"lookup_key"}` and `POST .../billing/portal` are
owners only, checked server side on every call, behind the same cookie and CSRF guard as the rest
of `/api/spaces`. Registre fills the checkout from its own state: the space id and name, the
owner's email, and return URLs on its issuer; the portal call carries the same space id and the
signed-in owner's email from the session, which Ardoise checks against the space's owners. It calls Ardoise with `Authorization: Bearer
BILLING_API_TOKEN`, a 5 s timeout and a 1 MiB response cap, and hands the page a URL only when its
origin is exactly `https://checkout.stripe.com` or `https://billing.stripe.com`. Ardoise down is
drawn as "billing unavailable" and answers 502 to a write. Registre caches the catalog for 60 s,
and the three plan routes share a per-person budget of one request a second with a burst of 20.

**A client may offer "Continue with Discord", and nobody else sees it.** `clients.login_providers`
(seed key `login_providers`, validated against the providers the build knows) is the opt-in:
`GET /api/login/providers?auth_request=` answers only what the request's client named and the
deployment has credentials for. `GET /api/login/social/{provider}/start` records a state (stored
as SHA-256, single use, ten minutes, a PKCE verifier beside it) and sets it as an HttpOnly Lax
cookie; the callback spends it only for the browser holding that cookie. The profile resolves in
order: a linked `(provider, subject)` signs that user in; a provider-verified email equal to an
existing address links to that account; any other verified email creates a passwordless account,
only while public sign-up is open. Disabled users are refused, and an account with a confirmed
second factor gets the same challenge a password login would. The `connections` scope adds the
claim `connections` = `{"discord": "<id>"}` to the ID token and userinfo, an empty object when
nothing is linked. Unlinking is refused when it would leave no password, passkey or other linked
account.

**Anyone may create a Facile account, when the owner allows it.** With `SIGNUP_ENABLED` on,
`/signup` takes an address and an ALTCHA proof of work. `POST /api/signup/start` verifies the
solution with the official `altcha-lib-go` in constant time, insists on a signed expiry no further
than ten minutes out, and records the challenge in `altcha_spent` so a solution is spent once. It
then answers `202 {"status":"sent"}` whether or not the address has an account, writing one
`signup_challenges` row either way (born spent for a known address) and sending one mail before it
answers: "you already have an account, sign in instead" for a known address, a link for an unknown
one. Both branches pay the same SMTP round trip, so waiting for it is no timing oracle, and a mail
the server refuses or cannot reach within ten seconds answers `503 {"error":"mail_failed"}` for
either branch, withdraws the row so the retry is not throttled, and is logged at ERROR with every
address redacted. A person is never told a mail is on its way when none is. The per-email limit
answers `429` with `retry_after` seconds and `Retry-After`. The page then says where the mail went,
who sends it, to look in spam, and that a known address gets sign-in instructions instead, and
offers a resend behind a sixty-second cooldown. An expired or spent link offers a new one in place.
The link's token is random, stored as SHA-256, good once for thirty minutes.
`/signup/finish` takes a name, a language and the acceptance of the current CGU and privacy
policy, then a passkey (registered before the account row exists, bound to the subject the
account is created with) or a password. The account, its consent rows, its passkey and the spent
link are one transaction, and the account is born verified (`email_verified_at`). Every route is
limited per address (an IPv6 address counted per /64); start is also limited to three links per
address per hour, counted from `signup_challenges` after a valid proof of work, so garbage
requests cannot spend a stranger's budget. Sign-up started from an app carries that app's login through only for the
browser that started it: `start` records the `auth_request` only when the browser presents the
flow cookie whose digest is that request's `browser_hash` (migration 00014, `internal/flows`),
and `finish` completes it only when the finishing browser presents the same cookie. The
callback then hands the code to that browser alone. A link opened in another browser, or a start
naming an auth request this browser did not open, lands on Registre's home with nothing
completed.

**Passwords follow NIST SP 800-63B-4.** Every path that sets one (sign-up, account page, reset,
invitation) requires 15 to 128 characters, counted as characters, with no composition rule.
Every new password on those paths is screened against Have I Been Pwned's range API (`HIBP_ENABLED`,
on by default): only five hex characters of
its SHA-1 leave the process, with `Add-Padding`, a 3 s timeout and a 1 MiB cap, and an outage
skips the check with a warning so sign-up never depends on a third party. Hashing stays
`internal/credentials` under the argon2 ceiling.

**Passkeys.** `internal/passkeys` is the WebAuthn relying party over `go-webauthn`, with
`WEBAUTHN_RP_ID` and exact `WEBAUTHN_ORIGINS` (defaults: the issuer's host and origin). The login
page offers a discoverable passkey sign-in that finishes like a password login with no second
factor owed, so it requires user verification: presence alone would let whoever holds an unlocked
security key in. Registration and reauthentication require it too. `/account` adds, renames and removes passkeys; the last
way into an account (its only passkey when it has no password) cannot be removed, and adding or
removing one needs a factor proven within ten minutes, either at sign-in or through
`/api/account/reauth`. Freshness is `sessions.factor_proven_at`, set when every factor the account
owes was proven (a sign-in or a reauthentication), never derived from when the session row was
created: the session a password change reissues is new but proves nothing. A password change
itself wants the current password and, when two-step sign-in is on, a live or recovery code. Each
ceremony is a `webauthn_ceremonies` row claimed once, bound to its purpose, its account where it
has one, and for sign-up the very link it was begun for. TOTP stays as it was.

**Every route that finishes a sign-in wants `X-Facile-CSRF`.** `POST /api/login`,
`/api/login/totp`, `/api/login/passkey/finish`, `/api/login/session` and `/api/signup/finish`
refuse a request without it with 403 before reading any credential. A cross-site page cannot set a
custom header without a preflight registre never answers, so it cannot sign a victim in to an
attacker's account (login CSRF). A login an access policy refuses opens no session row.

**Legal documents and consent.** `/legal/{cgu,cgv,privacy,legal-notice}?lang=xx` serves the
versioned texts in four languages from `GET /api/legal/{document}`; versions live in
`internal/consent`. An account must accept the current CGU and privacy policy: `identity.Complete`
refuses a login with `ErrConsentRequired` while an acceptance is owed, and every sign-in path then
sends the browser to `/consent`, which records the acceptance and resumes the login. A device
approval (`/device`) is held the same way. Bumping a version is how everyone is asked again. Each
acceptance is an appended `consents` row with the document, version, locale, time, a keyed digest
of the source address and whether the version was a draft.

**While the texts are drafts nothing is enforced.** `consent.Draft()` is true until a lawyer has
reviewed them; meanwhile `Outdated` answers nothing owed, so no existing person is held at the
consent screen for unreviewed terms. Sign-up still asks, and records the acceptance with `draft`
set; a draft acceptance never satisfies a reviewed version, so everyone is asked again once the
reviewed texts land with a new version. Consent is checked at interactive sign-in and device
approval only: a refresh grant keeps working until it expires (at most `REGISTRE_REFRESH_TOKEN_TTL`,
30 days) and a token exchange follows the subject token it trades, so an app that never sends the
person back to sign in learns of new terms at the latest a refresh lifetime later.

**GDPR.** `/account` exports the person's Registre data as JSON (no hashes, seeds or keys) and
schedules deletion after a 30-day grace period, behind a fresh factor and the address typed back;
the person can cancel until then. The only owner of a space other people use is refused
(`409 sole_owner`) until they make someone else owner; if they become sole owner again during the
grace period, erasure promotes the most senior other member (an admin first, then the earliest to
join) rather than withhold the erasure. The job, every ten minutes, ends every session, deletes
the account and every row keyed on it (invitations addressed to them included), fires back-channel
logout to the apps the sessions reached, bumps the directory version, and writes one
`deletion_notices` row per enabled application in the same transaction as the delete.

**`GET /api/directory/deletions` is the source of truth.** Every app lists its erasures for sixty
days with `?since=<RFC 3339>`, authenticated with HTTP Basic as its own application client like
the snapshot, and erases each listed `sub` within thirty days. An app that also wants a push
declares a `deletion_uri` (seed `deletion_uri`, or the dashboard's client page); only that URI ever
receives the event, never `directory_sync_uri`. A sync handler (porte's `Syncer.Handler`) answers
202 to any POST and only schedules a pull, so a push there would be counted delivered while
nothing erased anything. A notice counts as delivered only on a 2xx from the deletion URI:

```
POST <deletion_uri>
Content-Type: application/secevent+jwt

<JWT, RS256, typ secevent+jwt, signed with the key in the issuer's JWKS>
{
  "iss": "https://sso.facile.studio",
  "aud": ["<the app's client_id>"],
  "iat": 1762300000, "exp": 1762300600, "jti": "<random>",
  "sub": "<the deleted person's subject>",
  "events": { "https://schemas.facile.studio/event/user-deleted": { "erase_within_days": 30 } }
}
```

This is an RFC 8417 Security Event Token pushed as RFC 8935 describes. Before acting, the
receiving handler must verify, and answer 400 to anything else:

1. the JWS header `typ` is exactly `secevent+jwt` (a logout token or an ID token signed by the same
   key must never pass for a deletion);
2. the signature, against the issuer's JWKS by `kid`, RS256 only;
3. `iss` is the issuer, `aud` contains its own client id, `exp` is in the future;
4. `events` holds the key `https://schemas.facile.studio/event/user-deleted`;
5. `jti` has not been seen before (replays are refused).

It then answers 202 and erases `sub` within thirty days. Only a 2xx counts as delivered; a failed
push is retried after five minutes, then twice as long each time up to a day, twelve times at
most, and the notice stays listable either way. An app with no deletion URI, or one that was
down, learns from the list; an app that ignores both still pulls the snapshot, which no longer
lists the person, but keeps its data, which breaks the erasure promise.

**Space invitations.** `POST /api/spaces/{id}/members {"email","role"}` writes a
`space_invitations` row (fourteen days, at most fifty pending per space, rate limited per inviter
and per space) and answers `202 {"status":"invited"}` whatever the address. A person with an
account is mailed in their language; an address without one is mailed "join Facile" only while
sign-up is open. Neither mail carries any text a person typed, neither the inviter's name nor the
space's: both say only that someone sent an invitation, with a link to `/spaces` (or to sign up),
where the signed-in invitee reads who sent it, to what and as what. One address receives at most
three invitation mails a day, one inviter's invitations send at most twenty a day, and the
instance sends at most two hundred an hour, counted from a log of the mails actually sent
(`invitation_mails`), so refreshing an invitation cannot mail past it; past any cap the invitation is still
written and answered the same way, only the mail is skipped. Deleting a space deletes its pending
invitations, and an invitation into a deleted space can never be accepted. An invitation is the
inviter's authority held in escrow: accepting it needs the inviter still in the space at admin or
above and at least the offered rank, and removing, demoting or the departure of an inviter
withdraws the pending invitations they could no longer send. The invitee sees the space and the offered role on `/spaces` and accepts or
declines (`/api/spaces/invitations`); only acceptance writes `space_members`, so a pending
invitation never reaches a snapshot. Admins and owners see and revoke pending invitations.

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

**Decided 2026-08-24: the local administrator, restored by an offline subcommand.** The second
option was never really available. registre is the issuer, so it federates to nothing, and
"Authentik stays warm as a fallback" only helps an app that can be pointed at a second issuer,
not registre itself. `registre break-glass --email <address>` therefore takes one address and
puts it back into a state it can sign in from: re-enabled, second factor and recovery codes
removed, password set from `REGISTRE_BREAK_GLASS_PASSWORD`, admin granted, every live session and
refresh grant ended. It mints the account when the address has none, because the case this exists
for at its worst is the last administrator row deleted.

Three properties, each of which is the reason for the one before it:

- **It is a subcommand and must never become a route.** Running it already requires the database,
  so it grants nobody anything they did not have. Reachable over HTTP it would be a back door with
  a nicer name.
- **It is one transaction.** A half-applied restore is the worst outcome available: an account
  enabled with its factor cleared but its password unchanged is an account somebody else may now
  be able to reach, and the operator cannot tell from the error which half landed.
- **It is tested, which is the whole point.** `internal/breakglass` builds the worst state the
  account can be in and asserts one call answers all of it, including that no recovery code and no
  session survives. The password floor is 16 characters because this password is chosen under
  pressure by somebody improvising.

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

**An auth request id is not a credential, so the callback must not treat it as one.**
zitadel's `AuthorizeCallback` hands a code to whoever names a finished auth request. Left bare,
that made the id a bearer: an attacker started a flow at an app, had a signed-in victim finish it
(a link to the old `GET /api/login/session?auth_request=`), then called the callback and signed in
to the app as the victim. Three rules close it, all enforced in `modules/oidc/binding.go` and
`modules/login/session.go`:

- **The callback answers only the browser that started and finished the flow.** `/authorize`
  sets a per-flow HttpOnly cookie (`__Host-registre_flow_<digest>`, ten minutes) and stores its
  SHA-256 on the auth request. The callback requires that cookie and the account session whose id
  is `auth_requests.session_id`, or it answers `access_denied` and issues nothing.
- **One code per flow.** `SaveAuthCode` writes only into an empty `code`; a second callback is
  refused rather than minting a fresh code for whoever called last.
- **Finishing a flow is a POST with the CSRF header.** `POST /api/login/session` with
  `X-Facile-CSRF` and `{"auth_request": ...}` in the body. A GET only answers the "am I signed
  in" probe and refuses one that names a flow.

**A user code is low entropy, and the verification link puts it in a URL. Both are RFC 8628's
design, and neither contradicts the rule above.** Eight characters a human reads off one screen
and types into another is about 34.6 bits — §6.1's own worked example — and §5.2 requires the
endpoints that accept one to be rate limited. `verification_uri_complete` then embeds that code
in a link so a QR code can carry it. That is tolerable only because a user code is **not a
credential**: it names a pending request and authenticates nobody. Answering one still requires a
signed-in human, and `modules/device` copies the subject and the authentication time out of that
session rather than out of the link, so a page that could name the account it was approving for
would be an authorization bypass with a JSON body.

Three rules follow. The API enforces all three and the client is bound by all three.

- **One refusal, never a taxonomy.** Never issued, already answered, already spent and expired
  are one answer: `devices.ErrUnknownCode`, served as a single 404. Splitting them tells a
  guesser which of their guesses hit a real flow, which is the only thing guessing is for.
  `apps/client/src/lib/device.ts` renders that 404 as one outcome and may not widen it.
- **No code-shaped validation in the browser.** The page must not check length, alphabet or
  grouping before it sends. A form that refuses a shape locally is an oracle that costs an
  attacker nothing to query and never touches the rate limit. `devices.Normalise` on the server
  is what decides whether a code is a code.
- **Never submit without a human.** No lookup on a keystroke, and no approval on page load. A
  prefilled code is prefilled *into a field*; the human still presses the button. Otherwise a
  link is the approval, and RFC 8628's whole premise is that the approval is a deliberate act by
  somebody who knows what they started.

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

1. **Does the YAML seed cover human users?** **Answered 2026-08-24: yes it must, and it
   seeds invite tokens, never password hashes.** A fresh suite instance bootstrapped from files
   cannot require an operator to shell into the container to mint the first human. The invite
   token is a single-use, short-lived secret the seed writes only to the environment — the same
   discipline service-account secrets already follow (`secret_env`) — and its redemption sets
   the password through the normal login path under the argon2 ceiling. Implementation lands
   with self-service signup in phase 3's admin slice; until then every account is operator-
   minted via `registre user-create`, which marks the address verified by construction.
   **Self-service sign-up landed 2026-11-04** (§6): accounts are created only after the emailed
   link answers, so they are verified too.
2. **What happens to Authentik's two expression policies?** The avatar policy and the settings
   authorization policy in `authentik-config` are not part of the OIDC contract. Reimplement,
   or drop and accept the behaviour change. Decide before phase 3's admin UI.
   **Avatar answered 2026-11-03: reimplemented natively.** Registre stores the picture and emits
   `picture` (§6). The settings authorization policy is still open.
3. **Does porte need a second verifier to run dual-issuer?** `porte/oidc/oidc.go:118` builds one
   verifier from `ClientID`. Running Authentik and registre side by side during phase 4 may
   require porte to hold two. Verify early — it is the difference between a staged cutover and a
   flag day.
4. **Session lifetime and refresh policy.** Authentik's current values are unrecorded. Read them
   off the live instance *before* it is decommissioned.
5. **Does registre emit to `antenne`?** Login, logout and administrative changes are exactly the
   events the bus exists for, keyed on `actor_email`. Emit-only, if so.
6. **How does an app consume a machine token?** **Answered 2026-08-24: offline JWKS
   verification is the default; introspection is the opt-in.** Found while building phase 0, and it blocks the
   half of §4's exit that was struck through above. porte authenticates a bearer by hashing it and
   finding the row; a registre-signed JWT matches nothing. Two ways to close it, and the choice is
   porte's to make, in a version of its own:
   - **Verify offline.** porte grows a bearer path that, on a token shaped like a JWT, verifies it
     against the issuer's JWKS and checks `aud` against its own client id; anything else keeps
     hashing into `sessions`. Cache keys by `kid`; on an unknown `kid`, refetch the JWKS once,
     then refuse — being unable to verify is a reason to refuse, not to proceed. No round trip,
     no new failure mode at request time, and it inherits the key rotation phase 0 already proves.
     A revoked token works until expiry, which is what the short default lifetime is for; callers
     who need instant revocation use introspection instead and pay its round trip knowingly.
   - **Introspect.** porte calls `/oauth/introspect` with its own client credentials. Revocation
     is immediate and the app learns the scopes without parsing anything, at the cost of a round
     trip on every request and an identity provider that is now in the hot path of every app.
   Offline verification is the default because a per-request network hop to the IdP across all
   eleven apps buys nothing that short token lifetimes do not — that is the argon2-ceiling lesson:
   do not make one component an amplifier every caller depends on per-request. Whichever lands, the app-side identity is `(issuer, sub)` exactly as it is for a
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
