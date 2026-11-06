-- v0.5.1 review fixes.
--
-- clients.deletion_uri is the one place a signed user.deleted event is pushed.
-- It used to go to directory_sync_uri, whose handler in porte answers 202 to
-- any POST and only schedules a pull, so a notice was counted delivered while
-- nothing erased anything. Empty means the app learns of erasures only by
-- polling GET /api/directory/deletions, which stays the source of truth.
--
-- sessions.reauthenticated_at becomes factor_proven_at: the last time the
-- holder proved a factor, set at sign-in and at reauthentication but not when
-- a session is merely reissued, so freshness can no longer be inherited from
-- a session's creation time.
--
-- webauthn_ceremonies.bound_to ties a sign-up passkey ceremony to the link it
-- was begun for, and invitation_mails.inviter_id feeds a per-inviter cap.

-- +goose Up

ALTER TABLE clients ADD COLUMN IF NOT EXISTS deletion_uri TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions RENAME COLUMN reauthenticated_at TO factor_proven_at;
ALTER TABLE webauthn_ceremonies ADD COLUMN IF NOT EXISTS bound_to TEXT NOT NULL DEFAULT '';
ALTER TABLE invitation_mails ADD COLUMN IF NOT EXISTS inviter_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS invitation_mails_inviter ON invitation_mails (inviter_id, sent_at);

-- +goose Down

DROP INDEX IF EXISTS invitation_mails_inviter;
ALTER TABLE invitation_mails DROP COLUMN IF EXISTS inviter_id;
ALTER TABLE webauthn_ceremonies DROP COLUMN IF EXISTS bound_to;
ALTER TABLE sessions RENAME COLUMN factor_proven_at TO reauthenticated_at;
ALTER TABLE clients DROP COLUMN IF EXISTS deletion_uri;
