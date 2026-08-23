-- Phase 0: the register itself. Clients (service accounts, for now), the
-- signing keys their tokens are signed with, and a record of every token
-- issued so one can be revoked before it expires.
--
-- Human users, password identities and the auth code flow arrive in phase 1
-- and add their own migration; nothing here is written expecting to be the
-- last word on the schema.

-- +goose Up
CREATE TABLE IF NOT EXISTS clients (
    id               BIGSERIAL   PRIMARY KEY,
    client_id        TEXT        NOT NULL UNIQUE,
    subject          TEXT        NOT NULL UNIQUE,
    name             TEXT        NOT NULL DEFAULT '',
    kind             TEXT        NOT NULL DEFAULT 'service_account',
    secret_hash      TEXT        NOT NULL DEFAULT '',
    grant_types      JSONB       NOT NULL DEFAULT '[]'::jsonb,
    scopes           JSONB       NOT NULL DEFAULT '[]'::jsonb,
    audiences        JSONB       NOT NULL DEFAULT '[]'::jsonb,
    access_token_ttl BIGINT      NOT NULL DEFAULT 0,
    managed          BOOLEAN     NOT NULL DEFAULT false,
    disabled_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT clients_kind_known CHECK (kind IN ('service_account', 'application'))
);

CREATE TABLE IF NOT EXISTS signing_keys (
    id                TEXT        PRIMARY KEY,
    algorithm         TEXT        NOT NULL,
    public_key        BYTEA       NOT NULL,
    private_key       BYTEA       NOT NULL,
    private_key_nonce BYTEA       NOT NULL,
    state             TEXT        NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    activated_at      TIMESTAMPTZ,
    retired_at        TIMESTAMPTZ,
    expires_at        TIMESTAMPTZ,
    CONSTRAINT signing_keys_state_known CHECK (state IN ('published', 'active', 'retired'))
);

-- One active key at a time. The rotation is publish, then activate, then
-- retire the previous one, and a partial unique index is what stops two
-- concurrent replicas from both promoting their own published key.
CREATE UNIQUE INDEX IF NOT EXISTS signing_keys_one_active
    ON signing_keys (state) WHERE state = 'active';

CREATE INDEX IF NOT EXISTS signing_keys_expires ON signing_keys (expires_at);

CREATE TABLE IF NOT EXISTS access_tokens (
    id         TEXT        PRIMARY KEY,
    client_id  TEXT        NOT NULL,
    subject    TEXT        NOT NULL,
    scopes     JSONB       NOT NULL DEFAULT '[]'::jsonb,
    audiences  JSONB       NOT NULL DEFAULT '[]'::jsonb,
    issued_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS access_tokens_client ON access_tokens (client_id);
CREATE INDEX IF NOT EXISTS access_tokens_subject ON access_tokens (subject);
CREATE INDEX IF NOT EXISTS access_tokens_expires ON access_tokens (expires_at);

-- +goose Down
DROP TABLE IF EXISTS access_tokens;
DROP TABLE IF EXISTS signing_keys;
DROP TABLE IF EXISTS clients;
