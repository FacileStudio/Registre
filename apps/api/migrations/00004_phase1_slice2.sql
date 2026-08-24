-- Phase 1 slice two (refresh tokens, sessions) and phase 2 (roles,
-- back-channel logout, truthful email verification).
--
-- refresh_tokens holds one browser session's renewable credential each.
-- The table's key is the SHA-256 of the token value, not the value itself:
-- the refresh token crosses a redirect and can land in logs on every host
-- between the two, exactly like an authorization code.
--
-- sessions records one human login each, so end_session and a disabled
-- account have something concrete to terminate, and back-channel logout
-- knows which apps to notify.

-- +goose Up
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS roles JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS backchannel_logout_uri TEXT NOT NULL DEFAULT '';
ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS visible_roles JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE auth_requests
    ADD COLUMN IF NOT EXISTS session_id TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id         TEXT        PRIMARY KEY,
    user_id    TEXT        NOT NULL,
    client_id  TEXT        NOT NULL,
    scopes     JSONB       NOT NULL DEFAULT '[]'::jsonb,
    audiences  JSONB       NOT NULL DEFAULT '[]'::jsonb,
    amr        JSONB       NOT NULL DEFAULT '[]'::jsonb,
    auth_time  TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS refresh_tokens_user ON refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS refresh_tokens_expires ON refresh_tokens (expires_at);

CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT        PRIMARY KEY,
    user_id    TEXT        NOT NULL,
    client_ids JSONB       NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS sessions_user ON sessions (user_id);

-- +goose Down
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS refresh_tokens;
ALTER TABLE auth_requests DROP COLUMN IF EXISTS session_id;
ALTER TABLE clients DROP COLUMN IF EXISTS visible_roles;
ALTER TABLE clients DROP COLUMN IF EXISTS backchannel_logout_uri;
ALTER TABLE users DROP COLUMN IF EXISTS roles;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
