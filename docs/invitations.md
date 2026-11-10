# Space invitations

An owner or admin of a space invites an email address from `/spaces/{id}`. The
invitation is a `space_invitations` row; nobody joins until the invitee accepts.

## The mail

The invitee gets one mail per invitation, in their own language (the
inviter's language when the address has no account yet). Like GitHub's, it
names who invited them (display name, else email), the space and the role,
gives the expiry date, and has one button:

| Address | Button | Link |
|---|---|---|
| Has an account | View invitation / Voir l'invitation | `https://sso.facile.studio/invitations/{id}` |
| No account, sign-up open | Create my account and view the invitation | `/signup?invitation={id}&lang=xx`; the verification link carries `invitation`, so the new account lands on the invitation page |
| No account, sign-up closed | no mail | |

Every name in the mail is text someone typed. `i18n.Clean` drops control and
bidirectional-override characters (so nothing can start a header line) and
caps each value at 80 characters; `i18n.Mail` escapes it into the HTML part.
The mail has a text/plain part (`i18n.PlainText`) that spells out the link.
The anti-abuse caps are unchanged: 3 mails per recipient per day, 20 per
inviter per day, 200 per hour for the instance, 50 pending per space.

## The invitation page

`/invitations/{id}` shows the inviter, the space, the role and the expiry,
with Accept and Decline. Opening the page changes nothing: mail scanners
follow links, so only `POST /api/spaces/invitations/{id}/accept|decline`
(account cookie plus `X-Facile-CSRF`) answer an invitation. A `GET` on either
path answers 405.

`GET /api/spaces/invitations/{id}` drives the page:

| Status | Page shows |
|---|---|
| 200 | the invitation |
| 401 | sign in (`/login?next=/invitations/{id}`) or create an account |
| 403 | the invitation is for another address, and the signed-in email, with a switch-account button. Nothing about the space is returned |
| 410 | expired. Expired rows are kept 30 days so the page can say so |
| 404 | no longer available: withdrawn, already answered, the space was deleted, or the inviter lost the rank to offer it |

`/login?next=` accepts only `/invitations/{uuid}`; anything else is dropped.

## Pending invitations for suite apps

```
GET /api/directory/invitations?sub=<registre subject>
Authorization: Basic <app client_id:client_secret>
```

```json
{
  "count": 1,
  "invitations": [
    {
      "id": "0b9c1a2e-…",
      "space_name": "EpiKodi",
      "role": "member",
      "invited_by": "Noah",
      "expires_at": "2026-11-24T10:00:00Z",
      "url": "https://sso.facile.studio/invitations/0b9c1a2e-…"
    }
  ]
}
```

Only a first-party, confidential application client authenticates (the
same credentials and rule as `/api/directory/snapshot`). A service account,
a public client or a third-party client (CIMD, dynamic registration) gets
401. An unknown or disabled subject answers `{"count": 0, "invitations": []}`.
The endpoint only reads; accepting stays on Registre's page.

### Why HTTP Basic and not a user token

- It is how every syncing app already talks to Registre: the porte/directory
  credentials, no new grant or seed entry.
- A user access token is not reliably at hand. porte stores the sign-in
  access token, which expires after an hour, and refreshing it outside
  porte/exchange races refresh-token rotation. An exchanged token would need
  a `registre` audience in every app's seed `token_exchange` and
  porte/exchange wiring in the apps that lack it.
- The pending count is not in the directory snapshot on purpose: the
  snapshot goes to every app, so it would broadcast who has invitations; it
  would bump the directory version on every invite and re-sync every app;
  it cannot show expiry as it happens; and it would need a porte/directory
  release plus a column in every app.

The cost is one argon2 verification per call. Fetch it when the shell loads
and cache it per user for 60 seconds. Under load Registre answers 503 with
`Retry-After`; show no badge then.

## How an app wires it

1. Backend: add `GET /api/me/invitations` behind the app's session. Read the
   signed-in person's Registre subject (porte:
   `IdentityStore.ListByUser(ctx, identity.UserID)`, the row whose
   `Provider` is the issuer), then call Registre with the same issuer,
   client id and secret as the directory syncer:

   ```go
   req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
       issuer+"/api/directory/invitations?sub="+url.QueryEscape(subject), nil)
   req.SetBasicAuth(clientID, clientSecret)
   ```

   Answer `{count, href}` with `href` = `https://sso.facile.studio/spaces`
   when count is not 1, else the one invitation's `url`. Cache per user for
   60 seconds. Any error: answer `{count: 0}`; never fail the page for it.
2. Frontend: muse v0.18.0 or later. Pass the result to the user menu:

   ```svelte
   <SideBar userMenu={{ ...userMenu, invitations }} />
   <MobileNav userMenu={{ ...userMenu, invitations }} />
   ```

   with `invitations = { count, href }`, or `null` to show no row. muse
   draws the Invitations row, the count pill and the dot on the avatar.
3. Nothing to change in the seed or the app's CSP: the browser never calls
   Registre, and the link opens Registre in a new tab.
