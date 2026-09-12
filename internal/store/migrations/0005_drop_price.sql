-- +goose Up
-- Remove price watching. See plan/06-design-decisions.md D13.
--
-- Ticketmaster removed `priceRanges` from the Discovery API on 2025-03-11; minPrice
-- and maxPrice now always return null, at every access tier, and the replacement
-- (Inventory Status API) is partner-only. Measured against live data before doing
-- this: 0 of 60 search results and 0 of 20 single-event lookups carried price data,
-- while dates.status.code was present on 100/100.
--
-- So these columns could only ever hold NULL, and a price_below watch could never
-- fire. Rather than keep dead schema behind a feature that silently does nothing,
-- the app becomes an availability watcher.

-- Any price_below watches are unfireable by definition; convert them so the user's
-- intent ("alert me about this event") survives instead of silently dropping them.
UPDATE watches SET condition_type = 'becomes_available' WHERE condition_type <> 'becomes_available';

ALTER TABLE watches DROP CONSTRAINT IF EXISTS threshold_required_for_price_below;
ALTER TABLE watches DROP COLUMN IF EXISTS threshold_cents;

ALTER TABLE events DROP COLUMN IF EXISTS last_min_price_cents;
ALTER TABLE events DROP COLUMN IF EXISTS last_max_price_cents;

-- The table now records availability transitions only; the name should say so.
ALTER TABLE price_snapshots DROP COLUMN IF EXISTS min_price_cents;
ALTER TABLE price_snapshots DROP COLUMN IF EXISTS max_price_cents;
ALTER TABLE price_snapshots RENAME TO availability_snapshots;
ALTER INDEX IF EXISTS idx_snapshots_event_time RENAME TO idx_avail_snapshots_event_time;

-- +goose Down
ALTER INDEX IF EXISTS idx_avail_snapshots_event_time RENAME TO idx_snapshots_event_time;
ALTER TABLE availability_snapshots RENAME TO price_snapshots;
ALTER TABLE price_snapshots ADD COLUMN max_price_cents BIGINT;
ALTER TABLE price_snapshots ADD COLUMN min_price_cents BIGINT;

ALTER TABLE events ADD COLUMN last_max_price_cents BIGINT;
ALTER TABLE events ADD COLUMN last_min_price_cents BIGINT;

ALTER TABLE watches ADD COLUMN threshold_cents BIGINT;
ALTER TABLE watches ADD CONSTRAINT threshold_required_for_price_below
    CHECK (condition_type <> 'price_below' OR threshold_cents IS NOT NULL);
