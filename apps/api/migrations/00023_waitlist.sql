-- The product waitlists: Scribe and Pastille landing pages ask to be told
-- when the product ships.
--
-- One row per address and product, and nothing more than the form asks for:
-- the address, the product, the page's language and the version of the
-- consent line the person ticked. No address of origin, no user agent, no
-- name. A row is deleted, not flagged, when the person unsubscribes, so the
-- table only ever holds people who still want the mail.

-- +goose Up

CREATE TABLE IF NOT EXISTS waitlist_entries (
    id              TEXT PRIMARY KEY,
    email           TEXT NOT NULL,
    product         TEXT NOT NULL CHECK (product IN ('scribe', 'pastille')),
    locale          TEXT NOT NULL CHECK (locale IN ('fr', 'en', 'es', 'de')),
    consent_version TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS waitlist_entries_product_email ON waitlist_entries (product, email);

-- +goose Down

DROP TABLE IF EXISTS waitlist_entries;
