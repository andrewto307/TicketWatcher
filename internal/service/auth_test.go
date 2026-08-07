package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Register validates email/password before hashing or touching the DB, so those
// branches are testable with a nil queries handle.
func TestAuthService_RegisterValidation(t *testing.T) {
	s := NewAuthService(nil, "secret", time.Hour)
	ctx := context.Background()

	if _, err := s.Register(ctx, "not-an-email", "password123"); !errors.Is(err, ErrInvalidEmail) {
		t.Errorf("invalid email -> %v, want ErrInvalidEmail", err)
	}
	if _, err := s.Register(ctx, "a@b.com", "short"); !errors.Is(err, ErrWeakPassword) {
		t.Errorf("weak password -> %v, want ErrWeakPassword", err)
	}
}
