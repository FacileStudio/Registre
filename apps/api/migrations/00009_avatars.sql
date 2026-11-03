-- Profile pictures, one per human.
--
-- The bytes live in their own table rather than on users, because users is
-- read on every token issuance and a picture is up to a megabyte that none of
-- those reads want. users stays untouched, which also keeps the picture out of
-- every column-ownership argument the register has had.
--
-- id is random and replaced on every upload. It is the URL apps fetch, so a
-- new picture is a new URL: every cache in front of it, and every app that
-- stored the old one, sees the change without an invalidation step.
--
-- data is always a PNG that registre encoded itself. The upload is decoded and
-- re-encoded before it is stored, so no metadata and no trailing payload of the
-- original ever reaches this column.

-- +goose Up

CREATE TABLE IF NOT EXISTS avatars (
    id         TEXT        PRIMARY KEY,
    user_id    TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    data       BYTEA       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS avatars_one_per_user ON avatars (user_id);

-- +goose Down

DROP TABLE IF EXISTS avatars;
