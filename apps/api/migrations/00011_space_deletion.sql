-- Deleting a shared space.
--
-- A deleted space keeps its row with deleted_at set and loses every member.
-- It stays in the snapshot apps pull, with no members, so every app removes
-- everyone from its copy on the next sync and the space disappears from every
-- list in the suite. No app deletes its own data: files, events and tasks stay
-- where they are, owned by nobody registre syncs. Apps treat a space merely
-- absent from a snapshot as an accident to log, never as a deletion, which is
-- why deletion is spelled out here instead of by dropping the row.
--
-- The name is freed: only live spaces must have unique names.

-- +goose Up

ALTER TABLE spaces ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
DROP INDEX IF EXISTS spaces_one_name;
CREATE UNIQUE INDEX IF NOT EXISTS spaces_one_live_name ON spaces (lower(name)) WHERE deleted_at IS NULL;

-- +goose Down

DROP INDEX IF EXISTS spaces_one_live_name;
CREATE UNIQUE INDEX IF NOT EXISTS spaces_one_name ON spaces (lower(name));
ALTER TABLE spaces DROP COLUMN IF EXISTS deleted_at;
