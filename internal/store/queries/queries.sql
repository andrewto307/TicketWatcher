-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: UpsertEventByTMID :one
INSERT INTO events (tm_event_id, name, url, venue, event_date)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (tm_event_id) DO UPDATE
    SET name       = EXCLUDED.name,
        url        = EXCLUDED.url,
        venue      = EXCLUDED.venue,
        event_date = EXCLUDED.event_date
RETURNING *;

-- name: GetEvent :one
SELECT * FROM events WHERE id = $1;

-- name: GetEventByTMID :one
SELECT * FROM events WHERE tm_event_id = $1;

-- name: ListDueEvents :many
-- Events ready to poll that at least one active watch cares about.
SELECT e.*
FROM events e
WHERE e.next_poll_at <= now()
  AND EXISTS (SELECT 1 FROM watches w WHERE w.event_id = e.id AND w.status = 'active')
ORDER BY e.next_poll_at
LIMIT $1;

-- name: UpdateEventLatest :exec
UPDATE events
SET last_min_price_cents = $2,
    last_max_price_cents = $3,
    last_availability    = $4,
    last_polled_at       = now(),
    next_poll_at         = $5
WHERE id = $1;

-- name: MinPollIntervalForEvent :one
-- Effective cadence for an event = the smallest interval any active watch asked for.
SELECT COALESCE(MIN(poll_interval_s), 300)::int AS interval_s
FROM watches
WHERE event_id = $1 AND status = 'active';

-- name: InsertPriceSnapshot :one
INSERT INTO price_snapshots (event_id, min_price_cents, max_price_cents, availability_status)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: CreateWatch :one
INSERT INTO watches (user_id, event_id, condition_type, threshold_cents, poll_interval_s)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListActiveWatchesForEvent :many
-- Includes the owner's email so the notifier can address the alert.
SELECT w.*, u.email AS user_email
FROM watches w
JOIN users u ON u.id = w.user_id
WHERE w.event_id = $1 AND w.status = 'active';

-- name: ListWatchesWithEvent :many
SELECT w.*,
       e.tm_event_id,
       e.name AS event_name,
       e.venue,
       e.event_date,
       e.last_min_price_cents,
       e.last_max_price_cents,
       e.last_availability,
       e.last_polled_at
FROM watches w
JOIN events e ON e.id = w.event_id
WHERE w.user_id = $1
ORDER BY w.created_at DESC;

-- name: SetLastEvaluation :exec
-- Record the latest condition result (drives edge detection on the next poll).
UPDATE watches SET last_evaluation = $2 WHERE id = $1;

-- name: MarkNotified :exec
-- Stamp when we last alerted. The watch stays 'active' (pure edge-trigger):
-- it re-fires only on a new false->true transition, not while the condition stays true.
UPDATE watches SET last_notified_at = now() WHERE id = $1;

-- name: InsertNotification :one
INSERT INTO notifications (watch_id, channel, payload)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetWatch :one
SELECT * FROM watches WHERE id = $1 AND user_id = $2;

-- name: ListSnapshotsForEvent :many
SELECT * FROM price_snapshots WHERE event_id = $1 ORDER BY checked_at ASC;

-- name: UpdateWatch :one
-- Partial update: NULL args keep the current value. Editing the threshold re-arms
-- the watch (resets last_evaluation) so the new condition re-establishes its edge.
UPDATE watches
SET threshold_cents = COALESCE(sqlc.narg('threshold_cents'), threshold_cents),
    status          = COALESCE(sqlc.narg('status'), status),
    last_evaluation = CASE WHEN sqlc.arg('reset_evaluation') THEN false ELSE last_evaluation END
WHERE id = sqlc.arg('id') AND user_id = sqlc.arg('user_id')
RETURNING *;

-- name: DeleteWatch :execrows
DELETE FROM watches WHERE id = $1 AND user_id = $2;

-- name: ListNotificationsForUser :many
SELECT n.* FROM notifications n
JOIN watches w ON w.id = n.watch_id
WHERE w.user_id = $1
ORDER BY n.sent_at DESC
LIMIT $2;

-- name: MarkEventDue :exec
-- Make an event eligible for polling on the next scheduler tick. Called when a
-- new watch is created so it is evaluated promptly instead of waiting for the
-- event's next scheduled poll.
UPDATE events SET next_poll_at = now() WHERE id = $1;
