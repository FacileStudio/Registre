-- Shared spaces, owned by registre for the whole suite.
--
-- A space is a team ("Facile") that exists in every app with the same members
-- at the same rank. Apps no longer create spaces or invite people; they pull
-- this state and apply it. space_members.role is the suite ladder, owner >
-- admin > member, and an app maps it onto its own vocabulary.
--
-- directory_imports records which apps have handed over the spaces they held
-- before registre owned them. An import is accepted once per client, so an app
-- restarting can never bring back somebody an owner has since removed.
--
-- directory_state.version rises in the same transaction as every change, so an
-- app can skip a snapshot it already applied.

-- +goose Up

CREATE TABLE IF NOT EXISTS spaces (
    id         TEXT        PRIMARY KEY,
    name       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS spaces_one_name ON spaces (lower(name));

CREATE TABLE IF NOT EXISTS space_members (
    space_id   TEXT        NOT NULL REFERENCES spaces (id) ON DELETE CASCADE,
    user_id    TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       TEXT        NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (space_id, user_id)
);
CREATE INDEX IF NOT EXISTS space_members_by_user ON space_members (user_id);

CREATE TABLE IF NOT EXISTS directory_imports (
    client_id   TEXT        PRIMARY KEY,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS directory_state (
    id      INT    PRIMARY KEY CHECK (id = 1),
    version BIGINT NOT NULL
);
INSERT INTO directory_state (id, version) VALUES (1, 1) ON CONFLICT (id) DO NOTHING;

ALTER TABLE clients ADD COLUMN IF NOT EXISTS directory_sync_uri TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE clients DROP COLUMN IF EXISTS directory_sync_uri;
DROP TABLE IF EXISTS directory_state;
DROP TABLE IF EXISTS directory_imports;
DROP TABLE IF EXISTS space_members;
DROP TABLE IF EXISTS spaces;
