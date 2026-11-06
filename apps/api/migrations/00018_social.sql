-- Social sign-in: a Discord account can sign a person in to Registre.
--
-- social_identities links one external account to one Registre user. The pair
-- (provider, subject) is the key because a provider's subject is only unique
-- inside that provider, and (user_id, provider) is unique so a user holds at
-- most one account per provider and an unlink is unambiguous. Deleting a user
-- deletes their links.
--
-- social_states is the one-time state of a sign-in or link that has left for
-- the provider and not come back. Only the SHA-256 of the state is stored; the
-- browser holds the state itself in a cookie, so a callback finishes only in
-- the browser that started it. user_id is set when a signed-in user is
-- linking an account rather than signing in.
--
-- clients.login_providers is the opt-in list: a provider's button shows on a
-- login only when the client that sent the person here names it.

-- +goose Up

CREATE TABLE IF NOT EXISTS social_identities (
    provider     TEXT        NOT NULL,
    subject      TEXT        NOT NULL,
    user_id      TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    display_name TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, subject)
);
CREATE UNIQUE INDEX IF NOT EXISTS social_identities_one_per_user ON social_identities (user_id, provider);

CREATE TABLE IF NOT EXISTS social_states (
    state_hash      TEXT        PRIMARY KEY,
    provider        TEXT        NOT NULL,
    auth_request_id TEXT        NOT NULL DEFAULT '',
    user_id         TEXT        NOT NULL DEFAULT '',
    verifier        TEXT        NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS social_states_expires ON social_states (expires_at);

ALTER TABLE clients ADD COLUMN IF NOT EXISTS login_providers JSONB NOT NULL DEFAULT '[]';

-- +goose Down

ALTER TABLE clients DROP COLUMN IF EXISTS login_providers;
DROP TABLE IF EXISTS social_states;
DROP TABLE IF EXISTS social_identities;
