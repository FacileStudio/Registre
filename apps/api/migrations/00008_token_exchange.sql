-- The OAuth 2.0 token exchange grant, RFC 8693.
--
-- A relying party that holds a human's token (Bulle, acting for a signed-in
-- user) trades it for a short token addressed to exactly one other app. The
-- register decides who may do that and for which audience and scopes, so the
-- allowance is a column on the client rather than a rule in code: it is
-- declared in the seed, read at the token endpoint, and rewritten at every
-- boot like the rest of the declarative half of the row.
--
-- clients.token_exchange is a jsonb array of {"audience", "scopes"} entries.
-- An empty array means the client may not exchange, which is every client that
-- exists today.
--
-- access_tokens.actor records who a delegated token was issued to act as. It
-- is the client id of the party that exchanged, empty for every other grant.
-- The exchange refuses a subject token whose actor is set, which is what keeps
-- a delegation from being delegated again: the library's claims for a stored
-- token carry only the private ones, so the registered `act` claim is not
-- readable from the JWT and has to be remembered here.

-- +goose Up

ALTER TABLE clients ADD COLUMN IF NOT EXISTS token_exchange JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS actor TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE access_tokens DROP COLUMN IF EXISTS actor;
ALTER TABLE clients DROP COLUMN IF EXISTS token_exchange;
