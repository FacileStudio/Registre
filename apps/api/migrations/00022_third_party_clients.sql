-- Third-party OAuth clients and the grants people give them.
--
-- registration says where a client came from: '' for the seed and the admin
-- dashboard, 'metadata_document' for a client whose client_id is the URL of
-- its client ID metadata document (CIMD), and 'dynamic' for one registered
-- through RFC 7591. Only the last two need a person's consent before they get
-- a code. metadata_expires_at is when a fetched document must be read again.
--
-- client_grants holds one row per person, client and resource while the
-- grant is live. Revoking sets revoked_at, which frees the unique index for a
-- later grant, and keeps the row as the record of what was allowed.

-- +goose Up

ALTER TABLE clients ADD COLUMN IF NOT EXISTS registration TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN IF NOT EXISTS client_uri TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN IF NOT EXISTS metadata_expires_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS client_grants (
    id           TEXT PRIMARY KEY,
    user_id      TEXT        NOT NULL,
    client_id    TEXT        NOT NULL,
    resource     TEXT        NOT NULL,
    scopes       JSONB       NOT NULL DEFAULT '[]',
    created_at   TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS client_grants_live_idx
    ON client_grants (user_id, client_id, resource) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS client_grants_user_idx ON client_grants (user_id);

-- +goose Down

DROP TABLE IF EXISTS client_grants;
ALTER TABLE clients DROP COLUMN IF EXISTS metadata_expires_at;
ALTER TABLE clients DROP COLUMN IF EXISTS client_uri;
ALTER TABLE clients DROP COLUMN IF EXISTS registration;
