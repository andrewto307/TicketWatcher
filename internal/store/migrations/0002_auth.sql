-- +goose Up
-- Add a password hash for real accounts (Phase 5 auth). Existing rows (the seeded
-- demo user) get an empty hash and simply can't log in until they register.
ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN password_hash;
