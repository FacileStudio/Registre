# Plan: app access, per-app roles and a real admin dashboard

Checked against: SPEC §6 (the `roles` claim, filtered per client), CLAUDE.md "the register writes
only the columns a declaration owns", migrations (goose `-s`, own package, embedded), auth
(operator = Registre `admin` account over cookie session with `X-Facile-CSRF`, or HTTP Basic),
muse (tokens, Svelte 5 runes), filet, and the embedded client build (`scripts/build-client.sh`).

## Goal

Let an operator decide, per app, who may sign in and with which roles, see it all from one admin
dashboard, and keep a record of every admin change.

## Why (evidence)

- Any enabled user can sign into every application today: `identity.Complete`
  (`internal/identity/session.go`) binds the user to the auth request without asking which app it
  is for.
- Roles are one flat list per user (`users.roles`), intersected with `clients.visible_roles`
  (`modules/oidc/storage_claims.go`). There is no way to say "admin in Ardoise, member in Opus",
  and the admin UI cannot edit roles at all (`UpdateUser` sends only name and admin).
- The users table shows no pictures, no last sign-in and no app access. Nothing records who
  changed what.

## Approach

Copy the model the established IdPs converged on.

- An app is open to everyone until an operator restricts it (Authentik: "By default,
  applications are accessible to all users"; binding a user restricts it).
- A restricted app admits only the people granted to it (Zitadel's "check authorization on
  authentication").
- Roles live on the grant, per person and per app: Zitadel calls these user grants, Authentik
  calls them application entitlements.
- People are deactivated rather than deleted, and admin actions go to an activity log (common
  admin-console practice).

Concretely:

- New `app_grants` table (client, user, roles).
- New `clients.access_policy` column: `everyone` or `assigned`.
- One `internal/access` package answers "may this user use this app, and with which roles".
  Every path that hands a user a token for an app asks it.
- New `admin_events` table records admin changes.
- The admin UI gains user and app detail pages built on these.

## Steps (ordered)

1. `apps/api/migrations/00012_app_access.sql`. [migrations]
   - Add `clients.access_policy text not null default 'everyone'`, constrained to `everyone` or `assigned`.
   - Create `app_grants (client_id text references clients(client_id) on delete cascade, user_id text references users(id) on delete cascade, roles jsonb not null default '[]', granted_at, granted_by, primary key (client_id, user_id))`.
   - Create `admin_events (id, at, actor_id, action, target_type, target_id, detail jsonb)`.
   - Backfill each user's `roles` into grants for the clients whose `visible_roles` intersect them, then drop `users.roles`.
2. `apps/api/schemas/` — `AppGrant` and `AdminEvent` structs; `Client.AccessPolicy`; drop `User.Roles`.
3. `apps/api/internal/access/access.go` — `Allowed(ctx, clientID, userID)`, `Roles(ctx, clientID, userID)` (grant roles ∩ `visible_roles`), `Grant`, `Revoke`, `SetPolicy`.
   - Revoking access, or restricting an app, deletes the affected users' refresh tokens for that client, so access ends within one access-token lifetime.
   - Registre's own client can never be restricted, so an operator cannot lock the dashboard.
4. Enforcement. [auth]
   - `internal/identity/session.go` `Complete` refuses with `ErrNoAccess` when the user may not use the auth request's client. This covers the password, second-factor and session sign-ins.
   - `modules/login` answers 403 `{"error":"no_access","app":"<name>"}`, and the login page shows "You don't have access to <app>".
   - `modules/oidc/storage_refresh.go` refuses a refresh for a revoked user (`invalid_grant`).
   - `storage_exchange.go` refuses an exchange whose target audience the subject may not use.
   - `internal/devices` `Approve` refuses an unauthorized device login.
5. `modules/oidc/storage_claims.go` — the `roles` claim comes from `access.Roles`. SPEC §6 still holds: the claim stays flat and filtered per client. Amend SPEC in the same commit.
6. `internal/audit` — `Record(ctx, actor, action, target, detail)`. Call it from every `modules/admin` mutation: users, clients, grants, policy, secret rotation, invites.
7. `modules/admin` routes, all behind `requireOperator`, with `X-Facile-CSRF` on cookie mutations. [auth]
   - `GET /users` adds `picture_url`, `last_sign_in_at` and `apps`.
   - `GET /users/{id}` returns the user, their access per app (effective access, roles, grant) and their spaces.
   - `GET /clients/{id}/access` returns the policy and the grants with their users.
   - `PUT /clients/{id}/access` sets `{policy}`.
   - `PUT /clients/{id}/grants/{userID}` sets `{roles}`; `DELETE` on the same path revokes.
   - `GET /events?before=&limit=`.
   - Remove `roles` from `PUT /users/{id}`.
8. Client: `src/lib/admin.ts` gets the types and calls; `src/lib/access.ts` holds the pure helpers (effective access, role toggles) with a bun check.
9. UI. [muse]
   - Users list: avatar, name over email, Admin, Disabled and Unverified badges, app count, last sign-in, search, and a status filter (All, Active, Admins, Disabled).
   - User detail `/admin/users/[id]`: profile and actions, an "App access" list (one row per app: logo, policy, access toggle, role chips), and their spaces.
   - Apps list: logo, policy badge ("Everyone", or "Assigned · N"), status.
   - App detail `/admin/apps/[id]`:
     - Access (policy switch, the people with access and their roles, an add-people picker).
     - Configuration (the current form).
     - Danger zone (rotate the secret, disable, delete).
   - Activity tab: events with the actor's avatar, the action and the target.
10. `sh scripts/build-client.sh`, then commit `apps/api/internal/web/dist`.
11. Docs: SPEC §6 and §4, ROADMAP, `docs/api.md`, `docs/architecture.md`.

## Files to modify / new

- New:
  - `apps/api/migrations/00012_app_access.sql`
  - `apps/api/internal/access/`
  - `apps/api/internal/audit/`
  - `apps/api/modules/admin/access.go`
  - `apps/api/modules/admin/events.go`
  - `apps/client/src/routes/(app)/admin/users/[id]/`
  - `apps/client/src/routes/(app)/admin/apps/[id]/`
  - `apps/client/src/routes/(app)/admin/activity/`
  - `apps/client/src/lib/access.ts` and its check
- Changed:
  - `schemas/{client,user}.go`
  - `internal/identity/session.go`
  - `modules/login`
  - `modules/oidc/{storage_claims,storage_refresh,storage_exchange}.go`
  - `internal/devices/verify.go`
  - `modules/admin/{router,users,clients}.go`
  - the admin pages and `src/lib/admin.ts`
  - the docs listed in step 11

## Exit criteria

- `sh scripts/check.sh` and `filet check .` pass.
- Database tests (with `TEST_DATABASE_URL`) cover the following:
  - A restricted app refuses an ungranted user at `Complete`, at refresh, at exchange and at device approval.
  - The `roles` claim equals the grant's roles ∩ `visible_roles`.
  - Revoking a grant deletes that user's refresh tokens for the app.
  - Registre's own client refuses `assigned`.
  - Every admin mutation writes one event.
- In production:
  - Every existing app stays `everyone`, so nobody loses access on deploy.
  - Granting yann `admin` on Ardoise works from the dashboard.
  - The users list shows pictures.

## Risks / unknown unknowns

- Locking people out. The default stays `everyone`, and Registre's own client is unrestrictable, which covers that case.
- Bulle's token exchange audiences. A restricted target app now refuses delegated tokens for ungranted users. That is intended, but Bulle's tools will answer errors for those apps.
- Access tokens already issued stay valid until they expire. Revocation removes refresh tokens only, and SPEC's access-token TTL bounds the gap.
- The seed must never write `access_policy` (CLAUDE.md rule 5). The register keeps naming its columns explicitly.

## Skip (YAGNI)

- Groups and group-based assignment (Okta's recommendation). With four people, per-person grants are enough. Spaces already model teams, and binding a space to an app is the natural next step if it is ever needed.
- SCIM, sessions listing per user, bulk actions, CSV export, and audit retention policies.
- WebAuthn (ROADMAP defers it with a reason).

## Sources

- Authentik bindings: https://docs.goauthentik.io/add-secure-apps/bindings-overview/
- Zitadel projects and role checks: https://zitadel.com/docs/guides/manage/console/projects
- Zitadel token management: https://zitadel.com/blog/token-management-in-zitadel
- Zitadel audience caveat: https://help.zitadel.com/security-best-practices-validating-audience-aud-claims-in-zitadel-access-tokens
- Okta app assignment: https://help.okta.com/oie/en-us/Content/Topics/Provisioning/lcm/lcm-user-app-assign.htm
- Okta RBAC discussion: https://support.okta.com/help/s/question/0D54z00009TDgxHCAT/what-okta-groupsrbac-best-practices-are-you-using
- Deactivated users tab: https://retailersupport.happyreturns.com/user-management
- Admin panel practices: https://scriptagc.wasmer.app/https_codinginfinite_com/create-admin-panel-best-practices-tips
