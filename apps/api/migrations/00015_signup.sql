-- Public sign-up: locale, verified-at, consent, passkeys, address
-- challenges, proof-of-work replay guard and deletion requests.
--
-- Safe on a live database: every column is added with a default or as
-- nullable, every table is new, and the only UPDATE touches rows that exist
-- before sign-up does. Every account before this migration was minted by an
-- operator or an invitation, which attests the address, so each one is
-- backfilled as verified at its creation time.
--
-- Passkey-only accounts store an empty password_hash, which the existing
-- NOT NULL column already allows and which credentials.Verifier never
-- matches.

-- +goose Up

ALTER TABLE users ADD COLUMN IF NOT EXISTS locale TEXT NOT NULL DEFAULT 'en';
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_locale_known;
ALTER TABLE users ADD CONSTRAINT users_locale_known CHECK (locale IN ('fr', 'en', 'es', 'de'));
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;
UPDATE users SET email_verified = true, email_verified_at = created_at WHERE email_verified_at IS NULL;

ALTER TABLE sessions ADD COLUMN IF NOT EXISTS reauthenticated_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS consents (
    id          BIGSERIAL   PRIMARY KEY,
    user_id     TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    document    TEXT        NOT NULL,
    version     TEXT        NOT NULL,
    locale      TEXT        NOT NULL DEFAULT 'en',
    accepted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ip_hash     TEXT        NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS consents_user_document ON consents (user_id, document, accepted_at DESC);

CREATE TABLE IF NOT EXISTS webauthn_credentials (
    id               TEXT        PRIMARY KEY,
    user_id          TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    credential_id    BYTEA       NOT NULL,
    public_key       BYTEA       NOT NULL,
    attestation_type TEXT        NOT NULL DEFAULT '',
    transports       JSONB       NOT NULL DEFAULT '[]'::jsonb,
    aaguid           BYTEA,
    sign_count       BIGINT      NOT NULL DEFAULT 0,
    flags            SMALLINT    NOT NULL DEFAULT 0,
    name             TEXT        NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at     TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS webauthn_credentials_one_id ON webauthn_credentials (credential_id);
CREATE INDEX IF NOT EXISTS webauthn_credentials_user ON webauthn_credentials (user_id);

CREATE TABLE IF NOT EXISTS webauthn_ceremonies (
    id          TEXT        PRIMARY KEY,
    token_hash  TEXT        NOT NULL UNIQUE,
    purpose     TEXT        NOT NULL,
    user_id     TEXT,
    session     JSONB       NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT webauthn_ceremonies_purpose_known CHECK (purpose IN ('register', 'signup', 'login', 'reauth'))
);
CREATE INDEX IF NOT EXISTS webauthn_ceremonies_expires ON webauthn_ceremonies (expires_at);

CREATE TABLE IF NOT EXISTS signup_challenges (
    id              TEXT        PRIMARY KEY,
    email           TEXT        NOT NULL,
    token_hash      TEXT        NOT NULL UNIQUE,
    locale          TEXT        NOT NULL DEFAULT 'en',
    auth_request_id TEXT        NOT NULL DEFAULT '',
    expires_at      TIMESTAMPTZ NOT NULL,
    used_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS signup_challenges_expires ON signup_challenges (expires_at);
CREATE INDEX IF NOT EXISTS signup_challenges_email ON signup_challenges (lower(email));

CREATE TABLE IF NOT EXISTS altcha_spent (
    challenge  TEXT        PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS altcha_spent_expires ON altcha_spent (expires_at);

CREATE TABLE IF NOT EXISTS deletion_requests (
    user_id       TEXT        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    requested_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    execute_after TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS deletion_requests_due ON deletion_requests (execute_after);

-- +goose Down

DROP TABLE IF EXISTS deletion_requests;
DROP TABLE IF EXISTS altcha_spent;
DROP TABLE IF EXISTS signup_challenges;
DROP TABLE IF EXISTS webauthn_ceremonies;
DROP TABLE IF EXISTS webauthn_credentials;
DROP TABLE IF EXISTS consents;
ALTER TABLE sessions DROP COLUMN IF EXISTS reauthenticated_at;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_locale_known;
ALTER TABLE users DROP COLUMN IF EXISTS locale;
