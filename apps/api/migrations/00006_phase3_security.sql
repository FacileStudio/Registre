-- Phase 3, second slice: the second factor, the account session behind it,
-- and password recovery.
--
-- Four things arrive together because they are one story. A human enrols TOTP
-- from an account page, which needs a session registre itself owns rather than
-- one of its relying parties. Enrolling raises the cost of losing the device,
-- so recovery codes come in the same breath. And a factor that cannot be
-- recovered is a factor nobody sensible enables, so password recovery lands
-- with it.
--
-- Every secret here is stored the way the rest of the schema stores secrets:
-- the TOTP seed sealed under the root key like a signing key, and every
-- bearer token (session cookie, second-factor challenge, reset link) as a
-- SHA-256 digest, like an invitation. A dump of this database alone logs
-- nobody in.

-- +goose Up

-- The seed is sealed rather than hashed because it has to be read back to
-- verify a code. totp_confirmed_at separates an enrolment that was started
-- from one that was proven: a seed with no confirmation never gates a login,
-- so an abandoned enrolment cannot lock anyone out.
ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_secret BYTEA;
ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_nonce BYTEA;
ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_confirmed_at TIMESTAMPTZ;

-- RFC 6238 §5.2: a verifier must not accept a second code from the same time
-- step. Without this column a code shoulder-surfed or captured in flight is
-- replayable for the rest of its thirty seconds, which is the window an
-- attacker sitting on the wire actually has.
ALTER TABLE users ADD COLUMN IF NOT EXISTS totp_last_step BIGINT;

-- One row per unused code. They are single-use and hashed, so this table is
-- the same shape as invite_tokens for the same reasons.
CREATE TABLE IF NOT EXISTS totp_recovery_codes (
    id         TEXT        PRIMARY KEY,
    user_id    TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code_hash  TEXT        NOT NULL UNIQUE,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS totp_recovery_codes_user ON totp_recovery_codes (user_id);

-- The account cookie hangs off the session row that login already creates,
-- so ending a session, whether by end_session, user-disable or back-channel logout,
-- closes the account pages in the same statement that closes everything else.
-- A separate session table would have been a second thing to remember to
-- revoke, which is how a revocation gets missed.
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS token_hash TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS sessions_token_hash ON sessions (token_hash)
    WHERE token_hash IS NOT NULL;

-- A password checked but a second factor still owed. The row is the only
-- thing that carries that state: without it the browser would have to hold
-- "I got the password right", which is the whole factor handed to the client.
CREATE TABLE IF NOT EXISTS login_challenges (
    id              TEXT        PRIMARY KEY,
    auth_request_id TEXT        NOT NULL,
    user_id         TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash      TEXT        NOT NULL UNIQUE,
    attempts        INTEGER     NOT NULL DEFAULT 0,
    expires_at      TIMESTAMPTZ NOT NULL,
    consumed_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS login_challenges_expires ON login_challenges (expires_at);

-- Recovery is deliberately not an invitation: an invitation creates an
-- account and marks the address verified, and a reset must do neither. A
-- reset proves the address still reaches the person who already owns the row.
CREATE TABLE IF NOT EXISTS password_resets (
    id         TEXT        PRIMARY KEY,
    user_id    TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS password_resets_user ON password_resets (user_id);
CREATE INDEX IF NOT EXISTS password_resets_expires ON password_resets (expires_at);

-- +goose Down
DROP TABLE IF EXISTS password_resets;
DROP TABLE IF EXISTS login_challenges;
DROP INDEX IF EXISTS sessions_token_hash;
ALTER TABLE sessions DROP COLUMN IF EXISTS token_hash;
DROP TABLE IF EXISTS totp_recovery_codes;
ALTER TABLE users DROP COLUMN IF EXISTS totp_last_step;
ALTER TABLE users DROP COLUMN IF EXISTS totp_confirmed_at;
ALTER TABLE users DROP COLUMN IF EXISTS totp_nonce;
ALTER TABLE users DROP COLUMN IF EXISTS totp_secret;
