-- Phase 1 (first slice): human identities and the browser flow.
--
-- users holds the human principals: an email, an argon2id hash and nothing
-- derived from anything else. The email index is on lower(email) so a lookup
-- cannot miss because of capitalisation a user considers irrelevant.
--
-- auth_requests holds one pending browser flow each. It is deliberately its
-- own table rather than something in memory: registre restarts to deploy, and
-- dropping everyone mid-login every deploy is not "stateless", it is rude.
--
-- clients gains redirect_uris for application clients; service accounts never
-- start a browser flow, so theirs stays empty.

-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id            TEXT        PRIMARY KEY,
    email         TEXT        NOT NULL,
    name          TEXT        NOT NULL DEFAULT '',
    password_hash TEXT        NOT NULL,
    admin         BOOLEAN     NOT NULL DEFAULT false,
    disabled_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS users_one_email ON users (lower(email));

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS redirect_uris JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE IF NOT EXISTS auth_requests (
    id                    TEXT        PRIMARY KEY,
    client_id             TEXT        NOT NULL,
    redirect_uri          TEXT        NOT NULL,
    scopes                JSONB       NOT NULL DEFAULT '[]'::jsonb,
    state                 TEXT        NOT NULL DEFAULT '',
    nonce                 TEXT        NOT NULL DEFAULT '',
    response_type         TEXT        NOT NULL DEFAULT 'code',
    code_challenge        TEXT        NOT NULL DEFAULT '',
    code_challenge_method TEXT        NOT NULL DEFAULT '',
    user_id               TEXT,
    authenticated_at      TIMESTAMPTZ,
    expires_at            TIMESTAMPTZ NOT NULL,
    code                  TEXT        NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS auth_requests_expires ON auth_requests (expires_at);

-- +goose Down
DROP TABLE IF EXISTS auth_requests;
ALTER TABLE clients DROP COLUMN IF EXISTS redirect_uris;
DROP TABLE IF EXISTS users;
