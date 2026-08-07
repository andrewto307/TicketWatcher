package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ticket-watcher/internal/auth"
	"ticket-watcher/internal/store/db"
)

var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
	ErrInvalidEmail       = errors.New("a valid email is required")
)

// AuthService registers and authenticates users, issuing JWTs.
type AuthService struct {
	q      *db.Queries
	secret string
	ttl    time.Duration
}

func NewAuthService(q *db.Queries, secret string, ttl time.Duration) *AuthService {
	return &AuthService{q: q, secret: secret, ttl: ttl}
}

// Register creates an account and returns a signed token.
func (s *AuthService) Register(ctx context.Context, email, password string) (string, error) {
	email = normalizeEmail(email)
	if !strings.Contains(email, "@") {
		return "", ErrInvalidEmail
	}
	if len(password) < 8 {
		return "", ErrWeakPassword
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return "", err
	}
	u, err := s.q.CreateUser(ctx, db.CreateUserParams{Email: email, PasswordHash: hash})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return "", ErrEmailTaken
		}
		return "", err
	}
	return auth.NewToken(s.secret, u.ID, s.ttl)
}

// Login verifies credentials and returns a signed token.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	u, err := s.q.GetUserByEmail(ctx, normalizeEmail(email))
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrInvalidCredentials // don't reveal which of email/password was wrong
	}
	if err != nil {
		return "", err
	}
	if !auth.CheckPassword(u.PasswordHash, password) {
		return "", ErrInvalidCredentials
	}
	return auth.NewToken(s.secret, u.ID, s.ttl)
}

func normalizeEmail(e string) string { return strings.TrimSpace(strings.ToLower(e)) }
