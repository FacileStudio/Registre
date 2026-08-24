# Deployment

One container, one Traefik router, one hostname: `sso.facile.studio`. The compose file expects the
external `dokploy-network` and exposes nothing to the host.

```sh
cp .env.example .env      # fill in ENCRYPTION_KEY and POSTGRES_PASSWORD
docker compose up -d --build
```

## What happens at boot

1. Migrations run. A failure exits 1 rather than serving on a schema nobody described.
2. If the register has no *active* signing key, one is minted and activated immediately. On a
   fresh instance that skips the publish delay, which is safe exactly once: no relying party has
   ever fetched a key set, so there is no cache to be ahead of. On an instance that somehow lost
   its active key it is a recovery, and the alternative is a provider that cannot sign at all
   until somebody edits the database by hand.
3. The active key is opened, proving `ENCRYPTION_KEY` still matches what the database holds. A
   mismatch exits 1.
4. The seed is applied, if `REGISTRE_SEED_FILE` is set. Missing secrets fail the boot. Seeding in
   a container needs the file mounted and every `secret_env` it names present in the environment;
   `docker-compose.yml` carries a commented volume and a note, because neither can be guessed.
5. The rotation and token purge loops start.

`JOURNAL_URL` and `JOURNAL_TOKEN` are what put all of that, and every refusal the provider issues,
into the suite's log. See [configuration](configuration.md#log-shipping).

`docker compose exec registre-api /api migrate status` reports the schema version without starting
the server. `up` and `baseline` are there too, from `tronc/migrate`.

## TRUSTED_PROXIES behind Traefik

The login rate limiter keys on the request's source address, and behind Traefik that address is
Traefik's unless `TRUSTED_PROXIES` is configured so the real client IP is recovered from
`X-Forwarded-For`. Unset, every login from every app in the suite comes from one address and
shares one bucket of 5 requests per second — the first few sign-ins succeed and everything after
that is refused, which reads as a suite-wide login outage with nothing wrong in any app. Set
`TRUSTED_PROXIES` in production; the compose file carries the variable and a comment saying the
same.

## Health

`/health` is liveness, `/ready` checks the database. The container health check runs the binary's
own `healthcheck` argv hook, which is how a distroless image with no shell probes itself.

`/ready` is tronc's, at the root and under `/api`. The OIDC endpoints are mounted by name rather
than by mounting the provider at the root, because `zitadel/oidc` carries a `/ready` of its own
that would otherwise shadow the one the container's health check calls.

## Cutting over an app

Not yet. Phase 0 issues machine tokens and no app can consume one: porte authenticates a bearer by
hashing it and finding the row, and a signed JWT matches nothing. SPEC §10 question 6 is the two
ways to close that, and it is a porte change, not a registre one.

Authentik stays up and stays authoritative for human logins until phase 4. Both issuers can run at
once, and there is no moment where the suite has only an unproven identity provider.

## Secrets live in Casier

**As deployed 2026-08-23:** the secrets went into Dokploy's compose environment instead, because
the `casier` CLI was not available on the machine doing the deploy. The commands below are still
the target state; migrate `ENCRYPTION_KEY` and `POSTGRES_PASSWORD` into Casier before relying on
`casier check` for this project.

`casier.yml` declares the project (`registre`) and environment. The deployment's secrets belong
there rather than in a `.env` next to the compose file:

```sh
casier login
casier secrets set ENCRYPTION_KEY "$(openssl rand -base64 32)"
casier secrets set POSTGRES_PASSWORD "$(openssl rand -hex 24)"
casier secrets set JOURNAL_TOKEN journal_registre_...
casier secrets set REGISTRE_SEED_AGENT_CLAUDE_SECRET "$(openssl rand -hex 32)"
casier run -- docker compose up -d
```

Every `secret_env` a seed file names has to be set here too, by name. `casier check .env.example`
reports what is declared locally and missing remotely.

`ENCRYPTION_KEY` is the one that cannot be regenerated: see below.

## The one thing to back up

`ENCRYPTION_KEY`. Every signing key in the database is sealed under it, and the database without it
is a register of clients whose tokens can never be signed again.

Restoring a database taken under a different root key produces exactly that state, and it looks
healthy: every row is there, the JWKS still serves the public halves, `/ready` is green. registre
therefore opens its active signing key at boot and **exits 1** rather than starting, with an error
naming the variable. Before that guard existed the first symptom was a 500 on the first token
request saying `message authentication failed`, which names neither the variable nor the restore.
