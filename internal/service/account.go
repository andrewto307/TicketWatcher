package service

import (
	"context"
	"errors"

	"ticket-watcher/internal/auth"
	"ticket-watcher/internal/store/db"
)

var ErrAccountNotFound = errors.New("account not found")

// AccountService covers the Tier 3 obligations that come with emailing real
// people: a working opt-out, and the right to erase the account entirely.
type AccountService struct {
	q      *db.Queries
	secret string // signs/verifies unsubscribe tokens
}

func NewAccountService(q *db.Queries, secret string) *AccountService {
	return &AccountService{q: q, secret: secret}
}

// UnsubscribeURL builds the stable opt-out link embedded in every alert email.
func (s *AccountService) UnsubscribeURL(baseURL string, userID int64) string {
	return baseURL + "/api/unsubscribe?token=" + auth.NewUnsubscribeToken(s.secret, userID)
}

// Unsubscribe opts a user out of all alert email.
//
// It is idempotent, and it deliberately does not require a login: someone acting
// on an old email must be able to stop the mail immediately, without hunting for
// a password. That's both the legal expectation and the reason mailbox providers
// treat a broken unsubscribe as a spam signal.
func (s *AccountService) Unsubscribe(ctx context.Context, token string) error {
	userID, err := auth.ParseUnsubscribeToken(s.secret, token)
	if err != nil {
		return err
	}
	return s.q.SetUnsubscribed(ctx, userID)
}

// Resubscribe re-enables alerts for an authenticated user (undo, from the UI).
func (s *AccountService) Resubscribe(ctx context.Context, userID int64) error {
	return s.q.SetResubscribed(ctx, userID)
}

// Delete erases the account. Watches, snapshots, notifications and auth tokens
// all cascade from their foreign keys, so nothing is left behind.
func (s *AccountService) Delete(ctx context.Context, userID int64) error {
	n, err := s.q.DeleteUser(ctx, userID)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrAccountNotFound
	}
	return nil
}
