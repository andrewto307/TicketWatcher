-- +goose Up

CREATE TABLE users (
    id         BIGSERIAL   PRIMARY KEY,
    email      TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per unique Ticketmaster event. This is the poll target: we poll each
-- event once per cycle and fan the result out to every watch that references it.
-- Money is stored as integer cents (see plan/04-data-model.md).
CREATE TABLE events (
    id                   BIGSERIAL   PRIMARY KEY,
    tm_event_id          TEXT        NOT NULL UNIQUE,
    name                 TEXT        NOT NULL,
    url                  TEXT        NOT NULL DEFAULT '',
    venue                TEXT        NOT NULL DEFAULT '',
    event_date           TIMESTAMPTZ,
    -- latest observed state (denormalized for O(1) change detection + fast reads)
    last_min_price_cents BIGINT,
    last_max_price_cents BIGINT,
    last_availability    TEXT,
    last_polled_at       TIMESTAMPTZ,
    next_poll_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE watches (
    id               BIGSERIAL   PRIMARY KEY,
    user_id          BIGINT      NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    event_id         BIGINT      NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    condition_type   TEXT        NOT NULL,                    -- price_below | becomes_available
    threshold_cents  BIGINT,                                  -- required when condition_type='price_below'
    status           TEXT        NOT NULL DEFAULT 'active',   -- active | triggered | paused
    last_evaluation  BOOLEAN     NOT NULL DEFAULT false,      -- previous condition result (edge-trigger)
    last_notified_at TIMESTAMPTZ,
    poll_interval_s  INTEGER     NOT NULL DEFAULT 300,        -- desired cadence; event polls at MIN() across its watches
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT threshold_required_for_price_below
        CHECK (condition_type <> 'price_below' OR threshold_cents IS NOT NULL)
);

-- Time-series of observed prices, written on change (see plan/04-data-model.md).
CREATE TABLE price_snapshots (
    id                  BIGSERIAL   PRIMARY KEY,
    event_id            BIGINT      NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    min_price_cents     BIGINT,
    max_price_cents     BIGINT,
    availability_status TEXT,
    checked_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notifications (
    id       BIGSERIAL   PRIMARY KEY,
    watch_id BIGINT      NOT NULL REFERENCES watches(id) ON DELETE CASCADE,
    channel  TEXT        NOT NULL,                            -- email | sms
    sent_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload  JSONB       NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_events_next_poll         ON events (next_poll_at);
CREATE INDEX idx_watches_event            ON watches (event_id);
CREATE INDEX idx_watches_status           ON watches (status);
CREATE INDEX idx_snapshots_event_time     ON price_snapshots (event_id, checked_at DESC);
CREATE INDEX idx_notifications_watch_time ON notifications (watch_id, sent_at DESC);

-- Seed a single hardcoded user for v1 (real auth arrives in Phase 5).
INSERT INTO users (email) VALUES ('demo@example.com');

-- +goose Down
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS price_snapshots;
DROP TABLE IF EXISTS watches;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS users;
