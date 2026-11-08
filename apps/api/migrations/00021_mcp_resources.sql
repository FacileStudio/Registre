-- MCP resources and resource-bound browser flows (RFC 8707).
--
-- An application client may declare the MCP server it runs: its canonical URI
-- and the scopes a token for it can carry. A token requested for that URI is
-- minted with the URI as its only audience, so it is refused everywhere else.
-- The index keeps one owner per URI. auth_requests remembers which resource a
-- browser flow asked for, so the code exchange mints for exactly that one.

-- +goose Up

ALTER TABLE clients ADD COLUMN IF NOT EXISTS mcp_resource TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN IF NOT EXISTS mcp_scopes JSONB NOT NULL DEFAULT '[]';
CREATE UNIQUE INDEX IF NOT EXISTS clients_mcp_resource_idx ON clients (mcp_resource) WHERE mcp_resource <> '';
ALTER TABLE auth_requests ADD COLUMN IF NOT EXISTS resource TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE auth_requests DROP COLUMN IF EXISTS resource;
DROP INDEX IF EXISTS clients_mcp_resource_idx;
ALTER TABLE clients DROP COLUMN IF EXISTS mcp_scopes;
ALTER TABLE clients DROP COLUMN IF EXISTS mcp_resource;
