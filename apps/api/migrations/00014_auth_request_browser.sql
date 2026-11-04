-- Bind every browser flow to the browser that started it.
--
-- /authorize sets a short-lived flow cookie and stores the SHA-256 of its value
-- here. /authorize/callback hands out a code only to a browser that presents
-- that cookie and the account session that completed the flow, so an auth
-- request id is no longer a bearer. Rows created before this migration carry
-- the empty default and are refused at the callback; they live ten minutes.

-- +goose Up

ALTER TABLE auth_requests ADD COLUMN IF NOT EXISTS browser_hash TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE auth_requests DROP COLUMN IF EXISTS browser_hash;
