-- Plan entitlements, pushed by Ardoise and served in the directory snapshot.
--
-- Ardoise computes what each space and each person may do (storage, mailboxes,
-- Glouton access) and PUTs the whole picture to /api/directory/entitlements.
-- Registre stores it verbatim and hands it to every app with the spaces it
-- already syncs. "values" is a JSON object of feature key to a bool or an
-- integer limit, -1 meaning unlimited. Registre never interprets it.
--
-- A push replaces everything: a space or person absent from the request loses
-- its row. entitlements_state.revision is the last revision applied, so a
-- delayed or replayed push older than it is refused instead of rolling the
-- suite back to yesterday's plans.

-- +goose Up

CREATE TABLE IF NOT EXISTS space_entitlements (
    space_id   TEXT        PRIMARY KEY REFERENCES spaces (id) ON DELETE CASCADE,
    "values"   JSONB       NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_entitlements (
    user_id    TEXT        PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    "values"   JSONB       NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS entitlements_state (
    id       INT    PRIMARY KEY CHECK (id = 1),
    revision BIGINT NOT NULL
);
INSERT INTO entitlements_state (id, revision) VALUES (1, 0) ON CONFLICT (id) DO NOTHING;

-- +goose Down

DROP TABLE IF EXISTS entitlements_state;
DROP TABLE IF EXISTS user_entitlements;
DROP TABLE IF EXISTS space_entitlements;
