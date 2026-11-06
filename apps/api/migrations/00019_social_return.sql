-- Deep-link linking: an app can send a person through a provider and back.
--
-- social_states.return_to is the validated URL the browser goes back to once a
-- link finishes. Empty means the account page. The value is checked against
-- the redirect URIs of the clients that opted into the provider both when it
-- is stored and when it is used.

-- +goose Up

ALTER TABLE social_states ADD COLUMN IF NOT EXISTS return_to TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE social_states DROP COLUMN IF EXISTS return_to;
