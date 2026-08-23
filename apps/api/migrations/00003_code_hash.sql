-- Phase 1 follow-up: authorization codes are stored hashed, so the column
-- holds hex SHA-256 digests from this point on. Rows written by the previous
-- build hold plaintext, but they expire within minutes of issuance and are
-- burned on first use, so no data migration is needed — anything still
-- unhashed is unreachable within ten minutes of the deploy regardless.
--
-- The index backs AuthRequestByCode, the token endpoint's lookup.

-- +goose Up
CREATE INDEX IF NOT EXISTS auth_requests_code ON auth_requests (code);

-- +goose Down
DROP INDEX IF EXISTS auth_requests_code;