# Registre

The identity provider for the [Facile Suite](https://facile.studio). Self-hosted OIDC at
`sso.facile.studio`, replacing Authentik.

**Status: phase 0.** Machine tokens work end to end. There is no login page, no human user and no
app consuming a registre token yet. Read [SPEC.md](SPEC.md) before writing any code — it carries
the decisions, the contract registre has to satisfy, and the reasoning behind both.

Registre is an identity provider, not a package registry.

## What it does

Today:

- Issues machine tokens to service accounts over `client_credentials`, so an agent gets an
  identity of its own instead of borrowing a human's. The token is an RS256 JWT carrying the
  account's `sub`, the apps the register grants it as `aud`, and nothing it asked for
- Publishes a JWKS and rotates the key behind it without invalidating a token already issued
- Answers introspection and revocation, so a machine token can be taken back before it expires
- Bootstraps from a declarative YAML file that names its secrets rather than carrying them

Planned, in the order it will be built ([SPEC §4](SPEC.md)):

- Signs in people over `authorization_code` + PKCE, password first, then TOTP and WebAuthn
- Emits the suite's `roles` claim, filtered per application, and back-channel logout
- An administration dashboard, account recovery, and the migration off Authentik

## Stack

| Layer | Tech |
|---|---|
| API | Go 1.25, Chi v5, GORM, PostgreSQL 16, [tronc](https://github.com/FacileStudio/tronc) |
| OIDC | [`zitadel/oidc`](https://github.com/zitadel/oidc) — the protocol is not hand-rolled |
| Deploy | One distroless container behind Traefik, on la ruche |

## Quick start

```sh
mise install
cd apps/api && cp .env.example .env   # fill in ENCRYPTION_KEY
mise run dev
```

Needs a PostgreSQL 16; [docs/development.md](docs/development.md) has the one-line docker command
and the database the tests want.

## Configuration

`REGISTRE_ISSUER`, `ENCRYPTION_KEY` and `DATABASE_URL` are required; everything else has a
default. Two relationships are checked at boot rather than in production: the key retention must
outlast a token, and the publish delay must be shorter than the rotation interval. Both are the
same suite-wide 401, caught by arithmetic. See
[docs/configuration.md](docs/configuration.md).

Plans are off unless configured: `ENTITLEMENTS_WRITER` names the client allowed to push
entitlements, and `ARDOISE_BILLING_URL` with a `BILLING_API_TOKEN` of at least 32 characters
turns on the plan section of the spaces pages.

| Variable | Default | What it decides |
|---|---|---|
| `ENTITLEMENTS_WRITER` | unset | Client id allowed to `PUT /api/directory/entitlements`. Unset: 404 |
| `ARDOISE_BILLING_URL` | unset | Ardoise's API base. `https` outside development |
| `BILLING_API_TOKEN` | unset | Bearer for Ardoise, 32+ characters, server side only |

Public sign-up ("Create your Facile account") is off unless the owner turns it on. It also needs
SMTP, because every account starts with an emailed link.

| Variable | Default | What it decides |
|---|---|---|
| `SIGNUP_ENABLED` | `false` | Opens `/signup` and `/api/signup/*`. Off: every sign-up route answers 404 |
| `ALTCHA_HMAC_KEY` | unset | Signs the proof-of-work challenges. 32+ bytes or sign-up stays off. Server side only |
| `WEBAUTHN_RP_ID` | issuer host | The passkey relying party ID. Changing it orphans every registered passkey |
| `WEBAUTHN_ORIGINS` | issuer origin | Comma-separated exact origins passkeys may answer from, on the RP ID's domain, `https` |
| `HIBP_ENABLED` | `true` | Screens new passwords against Have I Been Pwned (5-character SHA-1 prefix only, 3 s timeout, skipped on outage) |

`OIDC_ISSUER` stays `https://sso.facile.studio` for every consuming app — the hostname was
deliberately made product-neutral so registre could move in behind it without any app changing a
variable.

## Structure

```
apps/api/
  main.go              boot: migrate, mint a key, seed, mount, serve
  migrations/          goose -s sequential, //go:embed *.sql
  schemas/             GORM models over what the migrations built
  modules/
    oidc/              the zitadel/oidc Storage implementation — the load-bearing package
  internal/
    keys/              signing keys, the JWKS, and the rotation lifecycle
    clients/           the register: who may ask for a token, and what with
    credentials/       argon2id, and the ceiling on how much of it runs at once
    seed/              the idempotent YAML bootstrap
```

A package under `modules/` owns HTTP and declares it in `router.go`; a domain service that owns no
routes lives under `internal/`. `filet.yml` enforces that with no exceptions.

## Documentation

- [SPEC.md](SPEC.md) — decisions, contract, phases, open questions
- [ROADMAP.md](ROADMAP.md) — what is done, what is left, and what is blocked on a human
- [docs/](docs/README.md) — architecture, configuration, development, deployment
- [porte/SPEC.md](https://github.com/FacileStudio/porte/blob/main/SPEC.md) — the client half,
  whose contract is frozen and which registre exists to satisfy

---

Part of the [Facile Suite](https://facile.studio) — self-hosted tools for creative studios
and freelancers. One login, zero cloud dependency.
# rebuild trigger Fri Sep  4 16:58:34 CEST 2026
# Manual rebuild trigger 1788536391
