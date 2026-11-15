-- How many Pastille charms a pre-order asks for. Optional: NULL means the
-- person did not say. Only the Pastille list takes a quantity; Scribe is a
-- single app, so a Scribe row never carries one.

-- +goose Up

ALTER TABLE waitlist_entries ADD COLUMN IF NOT EXISTS quantity SMALLINT;

ALTER TABLE waitlist_entries ADD CONSTRAINT chk_waitlist_entries_quantity
    CHECK (quantity IS NULL OR (quantity BETWEEN 1 AND 10 AND product = 'pastille'));

-- +goose Down

ALTER TABLE waitlist_entries DROP CONSTRAINT IF EXISTS chk_waitlist_entries_quantity;
ALTER TABLE waitlist_entries DROP COLUMN IF EXISTS quantity;
