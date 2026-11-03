-- Who may use which app, with which roles, and a record of admin changes.
--
-- An app is open to every enabled user until an operator restricts it:
-- access_policy 'everyone' keeps today's behaviour for every existing row, so
-- nobody loses access on deploy. 'assigned' admits only users holding a grant.
--
-- Roles move from one flat list per user to the grant, one list per user and
-- per app, so a person can be admin in one app and nothing in another. What
-- reaches a token is still the grant's roles intersected with the client's
-- visible_roles (SPEC §6). Each user's old roles are copied onto a grant for
-- every client that could see them, then the column goes.

-- +goose Up

ALTER TABLE clients ADD COLUMN IF NOT EXISTS access_policy TEXT NOT NULL DEFAULT 'everyone';
ALTER TABLE clients ADD CONSTRAINT clients_access_policy_known CHECK (access_policy IN ('everyone', 'assigned'));

CREATE TABLE IF NOT EXISTS app_grants (
    client_id  TEXT        NOT NULL REFERENCES clients (client_id) ON DELETE CASCADE,
    user_id    TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    roles      JSONB       NOT NULL DEFAULT '[]'::jsonb,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    granted_by TEXT        NOT NULL DEFAULT '',
    PRIMARY KEY (client_id, user_id)
);

CREATE INDEX IF NOT EXISTS app_grants_user_idx ON app_grants (user_id);

INSERT INTO app_grants (client_id, user_id, roles, granted_by)
SELECT c.client_id, u.id,
       (SELECT jsonb_agg(r) FROM jsonb_array_elements_text(u.roles) r WHERE c.visible_roles ? r),
       'migration'
FROM users u
JOIN clients c ON c.kind = 'application'
WHERE jsonb_typeof(u.roles) = 'array'
  AND EXISTS (SELECT 1 FROM jsonb_array_elements_text(u.roles) r WHERE c.visible_roles ? r)
ON CONFLICT DO NOTHING;

ALTER TABLE users DROP COLUMN IF EXISTS roles;

CREATE TABLE IF NOT EXISTS admin_events (
    id          BIGSERIAL   PRIMARY KEY,
    at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_id    TEXT        NOT NULL,
    action      TEXT        NOT NULL,
    target_type TEXT        NOT NULL,
    target_id   TEXT        NOT NULL,
    detail      JSONB       NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS admin_events_at_idx ON admin_events (at DESC, id DESC);

-- +goose Down

DROP TABLE IF EXISTS admin_events;
ALTER TABLE users ADD COLUMN IF NOT EXISTS roles JSONB;
DROP TABLE IF EXISTS app_grants;
ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_access_policy_known;
ALTER TABLE clients DROP COLUMN IF EXISTS access_policy;
