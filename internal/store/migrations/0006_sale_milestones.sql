-- +goose Up
-- Sale-milestone alerts. See plan/08-sale-milestone-alerts.md.
--
-- Ticketmaster's `sales` block (public window + presale windows) was always in the
-- API response; we simply weren't reading it. Storing it lets the app alert on
-- presales and announced onsale dates, not just the public-sale status flip.

-- The observed sale calendar for each event.
ALTER TABLE events ADD COLUMN public_onsale_at      TIMESTAMPTZ;  -- NULL when unknown/TBD
ALTER TABLE events ADD COLUMN public_onsale_end_at  TIMESTAMPTZ;
ALTER TABLE events ADD COLUMN earliest_presale_at   TIMESTAMPTZ;
ALTER TABLE events ADD COLUMN earliest_presale_name TEXT;
ALTER TABLE events ADD COLUMN presale_count         INTEGER NOT NULL DEFAULT 0;

-- True when the API returned its 9999-12-31 sentinel for the onsale date, i.e.
-- "not announced yet". Distinct from NULL-because-never-polled: measured at 4% of
-- events, and includes high-demand shows like Hamilton, so it needs its own state
-- rather than being conflated with "no data".
ALTER TABLE events ADD COLUMN onsale_tbd BOOLEAN NOT NULL DEFAULT false;

-- One row per (watch, alert kind) the user has already been told about.
--
-- A watch now has several independent things worth alerting on (presale opened,
-- public sale opened, onsale date announced, cancelled, …), so a single
-- last_notified_at can no longer express "which of these have fired".
--
-- The PK gives exactly-once semantics for free: the worker does
-- INSERT ... ON CONFLICT DO NOTHING and sends only when the insert wins. That
-- avoids the read-then-write race two pollers could otherwise hit.
CREATE TABLE watch_alerts (
    watch_id BIGINT      NOT NULL REFERENCES watches(id) ON DELETE CASCADE,
    kind     TEXT        NOT NULL,
    fired_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (watch_id, kind)
);

-- +goose Down
DROP TABLE IF EXISTS watch_alerts;
ALTER TABLE events DROP COLUMN IF EXISTS onsale_tbd;
ALTER TABLE events DROP COLUMN IF EXISTS presale_count;
ALTER TABLE events DROP COLUMN IF EXISTS earliest_presale_name;
ALTER TABLE events DROP COLUMN IF EXISTS earliest_presale_at;
ALTER TABLE events DROP COLUMN IF EXISTS public_onsale_end_at;
ALTER TABLE events DROP COLUMN IF EXISTS public_onsale_at;
