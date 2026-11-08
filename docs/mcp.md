# MCP authorization

Registre is the authorization server for every suite MCP server. An MCP client (Claude, ChatGPT,
Cursor, VS Code, MCP Inspector, Bulle) gets a token for one app's MCP server, and that token works
at that server only.

## What registre serves

| Spec | What it means here |
|---|---|
| RFC 8414 | `GET /.well-known/oauth-authorization-server` is the discovery document, with the same members |
| RFC 8707 | `resource` on `/authorize`, on the code and refresh grants, and on token exchange. The access token's `aud` is the resource URI alone |
| RFC 9207 | Every authorization response, success or error, carries `iss` |
| RFC 7636 | PKCE is S256 only. `code_challenge_method=plain` is refused with `invalid_request` |
| RFC 9068 | A resource-bound access token carries `scope`, space separated |

The token endpoint, discovery, AS metadata, JWKS and revocation answer CORS from any origin, with
no credentials. Token requests without a client secret have their own per-address budget
(`REGISTRE_PUBLIC_TOKEN_RATE`, default 20/s burst 200), because claude.ai users all refresh from
Anthropic's egress range.

## Token shape

A token minted for `https://courrier.facile.studio/mcp`:

```json
{
  "iss": "https://sso.facile.studio",
  "sub": "<registre user id>",
  "aud": ["https://courrier.facile.studio/mcp"],
  "client_id": "<the client that asked>",
  "scope": "courrier.read courrier.write",
  "exp": 1762600000, "iat": 1762599400, "nbf": 1762599400,
  "jti": "<token id>"
}
```

- `scope` lists only the scopes the resource declares. `openid` and `offline_access` never appear.
- A token Bulle obtained by token exchange also carries `act: {"sub": "bulle"}` and, when verified,
  `email`. A token without `act` came from a browser flow.
- A plain access token (no `resource`) is unchanged: `aud` is the client, and it has no `scope`.

## How an app adds its MCP server

1. Pick the URL people will paste into an MCP client, for example `https://opus.facile.studio/mcp`.
   It must be https, with no query and no fragment. Registre compares it byte for byte, so the MCP
   server's protected resource metadata must publish exactly this string as `resource`.
2. Add an `mcp` block to the app's entry in `seed.yaml`:

   ```yaml
   - client_id: aHKmbSdWbJKqUMdTjIqFz2h4YtKYkvMiTB39zr4D
     name: Opus
     # ...
     mcp:
       resource: https://opus.facile.studio/mcp
       scopes: [opus.read, opus.write]
   ```

   The scopes are `<name>.read` and `<name>.write`, with `name` lower-cased, the same vocabulary
   as `token_exchange`. The seed refuses anything else, and refuses one URL declared twice.
3. Deploy registre, then restart `registre-api` if the commit changes only `seed.yaml` (the seed is
   read at boot, and an unchanged image is not restarted).
4. In the app, verify tokens with porte's MCP verifier: issuer `https://sso.facile.studio`,
   audience the resource URI, scopes from the `scope` claim. Never accept the app's ordinary
   OIDC audience at `/mcp`, and never forward the token anywhere.
5. Bulle needs nothing new: its existing `token_exchange` grant for the app covers the app's MCP
   server. It sends `resource=<URI>` on the exchange and gets a token whose `aud` is the URI, with
   scopes limited to both its allowance and the resource's scopes.

## Rules registre enforces

- One resource per token. Two `resource` values are `invalid_target`.
- An unknown or disabled resource is `invalid_target`, at `/authorize` and at the token endpoint.
- A scope another app declares is `invalid_target` next to a resource, and a resource scope with
  no resource is `invalid_scope`.
- A code or refresh token is redeemed only for the resource it was issued for.
- The app behind a resource keeps its access policy: if it is restricted to assigned people, a
  token for its MCP server needs that assignment too, at sign-in, on refresh and on exchange.
