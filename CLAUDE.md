# Registre

The Facile Suite's own OIDC provider. Go + SvelteKit, deployed at `sso.facile.studio`.

**Read [SPEC.md](SPEC.md) first, every time.** It is the source of truth for what registre must
do and why. Where the code disagrees with it, the code is wrong until SPEC is amended in the same
commit.

## The three things most likely to be got wrong

1. **The contract is porte's, and it is frozen.** SPEC §6 lists what registre must emit. The
   `roles` claim filtered per client and back-channel logout are the two that break apps
   *silently* if missed — nothing errors, authorization just quietly stops working.
2. **Key rotation.** Publish a new key in JWKS before signing with it; keep the retired public
   key for a full token lifetime after last use. Getting this wrong 401s all eleven apps at once.
3. **Cutover is not reversible.** Fresh identities were chosen deliberately (SPEC §3), so the
   migration's verification pass — zero orphans, zero duplicate subjects — is the only safety
   net. SPEC §9.

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
- Call this a registry anywhere in prose. It is an identity provider; `Spore` is the package
  manager.
