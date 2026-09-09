package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ticket-watcher/internal/auth"
	"ticket-watcher/internal/notifier"
	"ticket-watcher/internal/store"
	"ticket-watcher/internal/store/db"
)

var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
	ErrInvalidEmail       = errors.New("a valid email is required")
	ErrInvalidToken       = errors.New("this link is invalid or has expired")
	ErrAlreadyVerified    = errors.New("email is already verified")
	ErrNoAccount          = errors.New("this account no longer exists")
)

// Token purposes (must match the CHECK constraint in migration 0003).
const (
	purposeVerifyEmail   = "verify_email"
	purposePasswordReset = "password_reset"
)

// Lifetimes of the emailed links. Verification is generous (people check mail
// late); reset is short because it's the higher-value credential.
const (
	verifyTokenTTL = 24 * time.Hour
	resetTokenTTL  = time.Hour
)

// Mailer sends transactional account email. notifier.Sender satisfies it, so the
// app reuses whichever channel is configured (Resend in prod, log-only in dev).
type Mailer interface {
	Send(ctx context.Context, msg notifier.Message) error
}

// AuthService registers and authenticates users, issuing JWTs, and drives the
// two email-confirmed flows (address verification and password reset).
type AuthService struct {
	q       *db.Queries
	secret  string
	ttl     time.Duration
	mail    Mailer // nil -> account emails are skipped (tests)
	baseURL string // public origin used to build emailed links
}

func NewAuthService(q *db.Queries, secret string, ttl time.Duration, mail Mailer, baseURL string) *AuthService {
	return &AuthService{q: q, secret: secret, ttl: ttl, mail: mail, baseURL: strings.TrimRight(baseURL, "/")}
}

// UserView is the authenticated user's own account state.
type UserView struct {
	ID            int64  `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Unsubscribed  bool   `json:"unsubscribed"`
}

// Register creates an account and returns a signed token. The account starts
// unverified and a confirmation email goes out; the user is logged in either way,
// so an email hiccup never blocks signup — it only withholds alert delivery.
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

	if err := s.sendVerification(ctx, u.ID, u.Email); err != nil {
		log.Printf("auth: verification email for user %d: %v", u.ID, err)
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

// Me returns the caller's own account state (drives the "verify your email" banner).
//
// A structurally valid token can outlive its account — the user deletes it, but
// the JWT stays in their browser until it expires. That's an authentication
// failure, not a server error, so it maps to ErrNoAccount and a 401; anything
// else would leave the frontend showing "internal error" instead of the login screen.
func (s *AuthService) Me(ctx context.Context, userID int64) (UserView, error) {
	u, err := s.q.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserView{}, ErrNoAccount
	}
	if err != nil {
		return UserView{}, err
	}
	return UserView{
		ID:            u.ID,
		Email:         u.Email,
		EmailVerified: u.EmailVerifiedAt.Valid,
		Unsubscribed:  u.UnsubscribedAt.Valid,
	}, nil
}

// ResendVerification re-sends the confirmation email to an authenticated user.
func (s *AuthService) ResendVerification(ctx context.Context, userID int64) error {
	u, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.EmailVerifiedAt.Valid {
		return ErrAlreadyVerified
	}
	return s.sendVerification(ctx, u.ID, u.Email)
}

// VerifyEmail redeems a verification token, marking the address confirmed.
func (s *AuthService) VerifyEmail(ctx context.Context, rawToken string) error {
	tok, err := s.redeemToken(ctx, rawToken, purposeVerifyEmail)
	if err != nil {
		return err
	}
	if err := s.q.MarkEmailVerified(ctx, tok.UserID); err != nil {
		return err
	}
	// Any other outstanding verification links are now pointless.
	_ = s.q.DeleteAuthTokensForUser(ctx, db.DeleteAuthTokensForUserParams{
		UserID: tok.UserID, Purpose: purposeVerifyEmail,
	})
	return nil
}

// RequestPasswordReset emails a one-time reset link.
//
// It reports success even when the address is unknown: a different answer for
// registered and unregistered emails would turn this endpoint into a way to
// enumerate who has an account.
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	u, err := s.q.GetUserByEmail(ctx, normalizeEmail(email))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	raw, err := s.issueToken(ctx, u.ID, purposePasswordReset, resetTokenTTL)
	if err != nil {
		return err
	}
	link := s.baseURL + "/reset-password?token=" + url.QueryEscape(raw)
	return s.send(ctx, notifier.PasswordResetMessage(u.Email, link))
}

// ResetPassword redeems a reset token and installs a new password.
func (s *AuthService) ResetPassword(ctx context.Context, rawToken, newPassword string) error {
	if len(newPassword) < 8 {
		return ErrWeakPassword
	}
	tok, err := s.redeemToken(ctx, rawToken, purposePasswordReset)
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.q.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{ID: tok.UserID, PasswordHash: hash}); err != nil {
		return err
	}
	// Burn every other outstanding reset link for this account.
	_ = s.q.DeleteAuthTokensForUser(ctx, db.DeleteAuthTokensForUserParams{
		UserID: tok.UserID, Purpose: purposePasswordReset,
	})
	return nil
}

// sendVerification issues a fresh verification token and emails the link.
func (s *AuthService) sendVerification(ctx context.Context, userID int64, email string) error {
	raw, err := s.issueToken(ctx, userID, purposeVerifyEmail, verifyTokenTTL)
	if err != nil {
		return err
	}
	// A GET link (not a form post) so clicking straight from an inbox works; the
	// handler verifies and redirects into the SPA.
	link := s.baseURL + "/api/auth/verify?token=" + url.QueryEscape(raw)
	return s.send(ctx, notifier.VerificationMessage(email, link))
}

// issueToken invalidates the user's outstanding tokens of this purpose, stores a
// new hashed one, and returns the raw value to embed in the email.
func (s *AuthService) issueToken(ctx context.Context, userID int64, purpose string, ttl time.Duration) (string, error) {
	if err := s.q.DeleteAuthTokensForUser(ctx, db.DeleteAuthTokensForUserParams{UserID: userID, Purpose: purpose}); err != nil {
		return "", fmt.Errorf("clear old %s tokens: %w", purpose, err)
	}
	raw, hash, err := auth.NewOpaqueToken()
	if err != nil {
		return "", err
	}
	if _, err := s.q.CreateAuthToken(ctx, db.CreateAuthTokenParams{
		UserID:    userID,
		TokenHash: hash,
		Purpose:   purpose,
		ExpiresAt: store.TS(time.Now().Add(ttl)),
	}); err != nil {
		return "", fmt.Errorf("store %s token: %w", purpose, err)
	}
	return raw, nil
}

// redeemToken looks up a raw token by its hash and marks it used. The query only
// matches unused, unexpired rows, so every failure mode collapses into
// ErrInvalidToken and nothing about the token's state leaks.
func (s *AuthService) redeemToken(ctx context.Context, rawToken, purpose string) (db.AuthToken, error) {
	if rawToken == "" {
		return db.AuthToken{}, ErrInvalidToken
	}
	tok, err := s.q.GetValidAuthToken(ctx, db.GetValidAuthTokenParams{
		TokenHash: auth.HashToken(rawToken),
		Purpose:   purpose,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.AuthToken{}, ErrInvalidToken
	}
	if err != nil {
		return db.AuthToken{}, err
	}
	if err := s.q.MarkAuthTokenUsed(ctx, tok.ID); err != nil {
		return db.AuthToken{}, err
	}
	return tok, nil
}

func (s *AuthService) send(ctx context.Context, msg notifier.Message) error {
	if s.mail == nil {
		return nil // no mailer configured (tests): nothing to do
	}
	return s.mail.Send(ctx, msg)
}

func normalizeEmail(e string) string { return strings.TrimSpace(strings.ToLower(e)) }
