-- +goose Up
-- Tier 3: alert opt-out. Anti-spam law (CAN-SPAM, CASL, GDPR) requires a working
-- unsubscribe in every message we send to a user's inbox.
--
-- NULL = still subscribed. This gates *alerts* only; transactional mail
-- (verification, password reset) still sends, because opting out of marketing
-- must not lock someone out of their own account.
ALTER TABLE users ADD COLUMN unsubscribed_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE users DROP COLUMN unsubscribed_at;
