-- Phase 3, first slice (the admin dashboard API).
--
-- invite_tokens holds one single-use invitation per row: a named email may
-- set its password once, before a deadline, through the normal login path
-- under the argon2 ceiling. The table's token column is the SHA-256 of the
-- token value and never the value itself — the plaintext is returned exactly
-- once at issue, like a client secret. SPEC §10 q1: the seed covers humans by
-- seeding invitations, never password hashes.

-- +goose Up
CREATE TABLE IF NOT EXISTS invite_tokens (
    id          TEXT        PRIMARY KEY,
    email       TEXT        NOT NULL,
    token_hash  TEXT        NOT NULL UNIQUE,
    created_by  TEXT        NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    redeemed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS invite_tokens_email ON invite_tokens (email);
CREATE INDEX IF NOT EXISTS invite_tokens_expires ON invite_tokens (expires_at);

-- +goose Down
DROP TABLE IF EXISTS invite_tokens;
