-- The OAuth 2.0 device authorization grant, RFC 8628.
--
-- A CLI on a machine with no browser — a server over SSH, a container — cannot
-- run the loopback flow: the browser that finishes the login is on a different
-- computer and 127.0.0.1 there is not 127.0.0.1 here. The device grant removes
-- the loopback entirely. The CLI asks for a pair of codes, shows the human the
-- short one, and polls the token endpoint with the long one while the human
-- approves it on whatever device has a browser.
--
-- Two codes, two threat models, two storage decisions.
--
-- The device code is a 128-bit random the CLI holds and presents at the token
-- endpoint. It is a bearer credential, so only its SHA-256 is stored, exactly
-- like sessions.token_hash, refresh_tokens.id and invite_tokens.token_hash. A
-- dump of this table logs nobody in.
--
-- The user code is eight characters a human reads off one screen and types
-- into another, which caps its entropy at about 34.6 bits (RFC 8628 §5.2 says
-- so plainly and requires the endpoint that accepts it to be rate limited).
-- It is stored in the normalised form the lookup uses — upper case, dashes
-- stripped — so that "bcdf-ghjk" and "BCDFGHJK" are the same row rather than a
-- near miss, and uniquely indexed so two live flows can never share one.
-- Hashing it would be theatre: 2^34.6 falls to an offline search in seconds,
-- so what actually defends it is the ten-minute expiry, the rate limit and the
-- fact that it is spent once.

-- +goose Up

CREATE TABLE IF NOT EXISTS device_codes (
    device_code_hash TEXT        PRIMARY KEY,
    user_code        TEXT        NOT NULL,
    client_id        TEXT        NOT NULL,
    scopes           JSONB       NOT NULL DEFAULT '[]'::jsonb,

    -- Who approved it, and how. Null until a human says yes: the row exists
    -- from the moment the CLI asks, and the poll answers authorization_pending
    -- off exactly this column being null.
    user_id          TEXT        REFERENCES users (id) ON DELETE CASCADE,
    amr              JSONB       NOT NULL DEFAULT '[]'::jsonb,
    auth_time        TIMESTAMPTZ,

    approved_at      TIMESTAMPTZ,
    denied_at        TIMESTAMPTZ,

    -- Stamped when the token endpoint hands the grant over. It is what makes
    -- the device code single use: an approved flow is redeemable exactly once,
    -- the way an authorization code is, so a device code captured in flight is
    -- worth nothing after the CLI that owns it has polled.
    redeemed_at      TIMESTAMPTZ,

    expires_at       TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Unique because a user code that names two flows would let a human approve
-- one device and hand the tokens to another. Low entropy makes a collision far
-- likelier than it would be for any other credential here, which is why the
-- constraint is in the schema and not in the code that mints them.
CREATE UNIQUE INDEX IF NOT EXISTS device_codes_user_code ON device_codes (user_code);

-- Expired flows are deleted rather than kept, so the table holds roughly the
-- last hour of activity and the odds of a user code colliding stay where the
-- entropy calculation says they are. zitadel/oidc's own interface documentation
-- requires this: "implementers of this interface must make sure that user codes
-- of expired authentication flows are purged".
CREATE INDEX IF NOT EXISTS device_codes_expires ON device_codes (expires_at);

-- A public client holds no secret, which is the only honest shape for a CLI
-- installed on a laptop: anything shipped with the binary is readable by
-- whoever has the binary, and calling it a secret does not make it one. The
-- column is explicit rather than derived from an empty secret_hash, because
-- "this row has no credential" and "this row's credential failed to seed"
-- would then be the same state, and one of them must never authenticate.
ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS public BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE clients DROP COLUMN IF EXISTS public;
DROP TABLE IF EXISTS device_codes;
