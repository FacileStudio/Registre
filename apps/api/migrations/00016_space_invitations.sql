-- Space invitations: adding a person to a space asks them first.
--
-- Before this, an owner or admin typing an address put that account straight
-- into the space. With public sign-up that is a way to push strangers into
-- spaces they never asked for. Now it writes a pending invitation here, the
-- person accepts or declines on /spaces, and only acceptance creates the
-- space_members row. The directory snapshot reads only space_members, so a
-- pending invitation never reaches an app.
--
-- Existing memberships are untouched: nothing here reads or writes
-- space_members.

-- +goose Up

CREATE TABLE IF NOT EXISTS space_invitations (
    id         TEXT        PRIMARY KEY,
    space_id   TEXT        NOT NULL REFERENCES spaces (id) ON DELETE CASCADE,
    email      TEXT        NOT NULL,
    role       TEXT        NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    invited_by TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS space_invitations_one ON space_invitations (space_id, lower(email));
CREATE INDEX IF NOT EXISTS space_invitations_email ON space_invitations (lower(email));
CREATE INDEX IF NOT EXISTS space_invitations_expires ON space_invitations (expires_at);

-- One row per invitation mail sent, so the per-recipient and hourly caps
-- count mails rather than invitations: refreshing an invitation rewrites its
-- row and would otherwise mail without limit. Purged after a day.
CREATE TABLE IF NOT EXISTS invitation_mails (
    id      BIGSERIAL   PRIMARY KEY,
    email   TEXT        NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS invitation_mails_email ON invitation_mails (lower(email), sent_at);
CREATE INDEX IF NOT EXISTS invitation_mails_sent ON invitation_mails (sent_at);

-- +goose Down

DROP TABLE IF EXISTS invitation_mails;
DROP TABLE IF EXISTS space_invitations;
