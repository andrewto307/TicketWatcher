-- +goose Up
-- Tier 1: email verification + password reset.

-- NULL = unverified. Alert emails are only sent to verified addresses so we never
-- mail someone who never confirmed they own the account.
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;

-- Grandfather everyone who registered before verification existed (including the
-- seeded demo user). Without this, live accounts would silently stop receiving
-- alerts the moment this migration lands.
UPDATE users SET email_verified_at = now() WHERE email_verified_at IS NULL;

-- Single-use, expiring tokens for the two email-driven flows.
--
-- Only the SHA-256 hash is stored: the raw token exists solely in the email we
-- send, so a database leak can't be replayed to take over accounts (same reasoning
-- as storing bcrypt hashes instead of passwords).
CREATE TABLE auth_tokens (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL UNIQUE,
    purpose    TEXT        NOT NULL,             -- verify_email | password_reset
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,                      -- non-NULL once redeemed (single use)
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT auth_tokens_purpose_valid
        CHECK (purpose IN ('verify_email', 'password_reset'))
);

-- Supports "invalidate this user's outstanding tokens for a purpose", which we do
-- whenever a new one is issued and after a successful redemption.
CREATE INDEX idx_auth_tokens_user_purpose ON auth_tokens (user_id, purpose);

-- +goose Down
DROP TABLE IF EXISTS auth_tokens;
ALTER TABLE users DROP COLUMN email_verified_at;
