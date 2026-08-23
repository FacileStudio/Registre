# Registre

The identity provider for the [Facile Suite](https://facile.studio). Self-hosted OIDC at
`sso.facile.studio`, replacing Authentik.

**Status: specified, not built.** Read [SPEC.md](SPEC.md) before writing any code — it carries
the decisions, the contract registre has to satisfy, and the reasoning behind both.

Registre is an identity provider, not a package registry. `Spore` is the suite's package manager.

## What it does

Nothing yet. Planned, in the order it will be built:

- Issues machine tokens to service accounts over `client_credentials`, so agents get their own
  identity instead of borrowing a human's
- Signs in people over `authorization_code` + PKCE, password first, then TOTP and WebAuthn
- Emits the suite's `roles` claim, filtered per application, and back-channel logout
- Bootstraps a fresh suite instance from declarative YAML, keeps runtime state in Postgres, and
  is administered from a dashboard

## Stack

| Layer | Tech |
|---|---|
| API | Go 1.25, Chi v5, GORM, PostgreSQL 16, [tronc](https://github.com/FacileStudio/tronc) |
| OIDC | [`zitadel/oidc`](https://github.com/zitadel/oidc) — the protocol is not hand-rolled |
| Client | SvelteKit 5 (runes), Tailwind CSS 4, muse |
| Deploy | One distroless container behind Traefik, on la ruche |

## Quick start

There is no code to run. The next commit is phase 0 in [SPEC.md](SPEC.md) §4.

## Configuration

Not yet fixed. `OIDC_ISSUER` stays `https://sso.facile.studio` for every consuming app — the
hostname was deliberately made product-neutral so registre could move in behind it without any
app changing a variable.

## Documentation

- [SPEC.md](SPEC.md) — decisions, contract, phases, open questions
- [porte/SPEC.md](https://github.com/FacileStudio/porte/blob/main/SPEC.md) — the client half,
  whose contract is frozen and which registre exists to satisfy

---

Part of the [Facile Suite](https://facile.studio) — self-hosted tools for creative studios
and freelancers. One login, zero cloud dependency.
