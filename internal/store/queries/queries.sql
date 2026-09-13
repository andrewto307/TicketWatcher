-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING *;

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified_at = now() WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: CreateAuthToken :one
-- token_hash is the SHA-256 of the token we emailed; the raw value is never stored.
INSERT INTO auth_tokens (user_id, token_hash, purpose, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetValidAuthToken :one
-- Redeemable only while unused and unexpired, so lookup failure is indistinguishable
-- between "wrong", "already used", and "expired" — nothing leaks to the caller.
SELECT * FROM auth_tokens
WHERE token_hash = $1
  AND purpose    = $2
  AND used_at   IS NULL
  AND expires_at > now();

-- name: MarkAuthTokenUsed :exec
UPDATE auth_tokens SET used_at = now() WHERE id = $1;

-- name: DeleteAuthTokensForUser :exec
-- Invalidate outstanding tokens of one purpose: called when issuing a new one and
-- after a successful redemption, so an old link in an inbox can't be replayed.
DELETE FROM auth_tokens WHERE user_id = $1 AND purpose = $2;

-- name: CountWatchesForUser :one
SELECT count(*) FROM watches WHERE user_id = $1;

-- name: SetUnsubscribed :exec
-- Idempotent on purpose: clicking an unsubscribe link twice must not error.
UPDATE users SET unsubscribed_at = now() WHERE id = $1;

-- name: SetResubscribed :exec
UPDATE users SET unsubscribed_at = NULL WHERE id = $1;

-- name: DeleteUser :execrows
-- Right to erasure. watches -> availability_snapshots/notifications cascade from the
-- FKs in 0001_init, and auth_tokens cascades from 0003, so this removes every
-- trace of the account in one statement.
DELETE FROM users WHERE id = $1;

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
SET last_availability     = $2,
    public_onsale_at      = $3,
    public_onsale_end_at  = $4,
    earliest_presale_at   = $5,
    earliest_presale_name = $6,
    presale_count         = $7,
    onsale_tbd            = $8,
    last_polled_at        = now(),
    next_poll_at          = $9
WHERE id = $1;

-- name: MinPollIntervalForEvent :one
-- Effective cadence for an event = the smallest interval any active watch asked for.
SELECT COALESCE(MIN(poll_interval_s), 300)::int AS interval_s
FROM watches
WHERE event_id = $1 AND status = 'active';

-- name: InsertAvailabilitySnapshot :one
INSERT INTO availability_snapshots (event_id, availability_status)
VALUES ($1, $2)
RETURNING *;

-- name: CreateWatch :one
-- last_evaluation is seeded from the event's current state: if it is already on
-- sale when the user subscribes, that is not a rising edge and must not alert.
INSERT INTO watches (user_id, event_id, condition_type, poll_interval_s, last_evaluation)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListActiveWatchesForEvent :many
-- Includes the owner's email so the notifier can address the alert, plus the two
-- states that decide whether we're allowed to mail them at all: verification
-- (did they confirm the address?) and opt-out (did they unsubscribe?).
SELECT w.*, u.email AS user_email, u.email_verified_at, u.unsubscribed_at
FROM watches w
JOIN users u ON u.id = w.user_id
WHERE w.event_id = $1 AND w.status = 'active';

-- name: ListWatchesWithEvent :many
SELECT w.*,
       e.tm_event_id,
       e.name AS event_name,
       e.venue,
       e.url AS event_url,
       e.event_date,
       e.last_availability,
       e.last_polled_at,
       e.public_onsale_at,
       e.public_onsale_end_at,
       e.earliest_presale_at,
       e.earliest_presale_name,
       e.presale_count,
       e.onsale_tbd
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
SELECT * FROM availability_snapshots WHERE event_id = $1 ORDER BY checked_at ASC;

-- name: UpdateWatch :one
-- Partial update: a NULL status keeps the current value. reset_evaluation re-arms the
-- watch so it can fire again on the next false->true edge.
UPDATE watches
SET status          = COALESCE(sqlc.narg('status'), status),
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

-- name: ClaimWatchAlert :execrows
-- Exactly-once gate for one (watch, kind) alert. Returns 1 the first time and 0
-- thereafter, so the caller sends the email only when it wins the insert. Doing
-- this as a single statement avoids the read-then-write race a SELECT-then-INSERT
-- would leave open.
INSERT INTO watch_alerts (watch_id, kind)
VALUES ($1, $2)
ON CONFLICT (watch_id, kind) DO NOTHING;

-- name: ListFiredAlertKinds :many
SELECT kind FROM watch_alerts WHERE watch_id = $1;

-- name: CloseWatchesForEvent :execrows
-- Pause every active watch on an event (used when the event date has passed, so
-- the scheduler stops spending API budget on it).
UPDATE watches SET status = 'paused' WHERE event_id = $1 AND status = 'active';
