-- Per-account failed-login throttle.
--
-- One row per throttled key: the SHA-256 of a kind and a normalised identity,
-- never the address itself, so the table holds nothing a leak could read back.
-- Rows exist for unknown addresses on purpose, otherwise "no row" would tell a
-- guesser which addresses are real. The purge removes a row a day after its
-- last attempt.

-- +goose Up

CREATE TABLE IF NOT EXISTS login_throttles (
    key             TEXT PRIMARY KEY,
    failures        INTEGER     NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ NOT NULL,
    next_allowed_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS login_throttles_last_attempt_idx ON login_throttles (last_attempt_at);

-- +goose Down

DROP TABLE IF EXISTS login_throttles;
