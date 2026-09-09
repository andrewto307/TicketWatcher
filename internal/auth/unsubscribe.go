package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

// Unsubscribe links are different from verification and reset links in two ways
// that rule out the auth_tokens table:
//
//   - They must work **forever**. The link sits in every alert email ever sent;
//     a recipient may act on a year-old message, and anti-spam law expects that
//     to still work.
//   - They must be **reusable**. Single-use would break the second click.
//
// So instead of storing anything, the token is the user id plus an HMAC of it
// keyed by the app secret: stateless, stable, verifiable, and unforgeable
// without the secret. Nothing sensitive is exposed — a user id is not a
// credential, and the MAC is what makes it authoritative.

var ErrInvalidUnsubscribeToken = errors.New("invalid unsubscribe token")

// NewUnsubscribeToken returns a stable opt-out token for the user.
func NewUnsubscribeToken(secret string, userID int64) string {
	id := strconv.FormatInt(userID, 10)
	return id + "." + unsubscribeMAC(secret, id)
}

// ParseUnsubscribeToken verifies a token and returns the user id it names.
func ParseUnsubscribeToken(secret, token string) (int64, error) {
	id, mac, ok := strings.Cut(token, ".")
	if !ok {
		return 0, ErrInvalidUnsubscribeToken
	}
	// Constant-time compare: a timing-variable check would leak the MAC one byte
	// at a time to an attacker willing to make enough requests.
	if !hmac.Equal([]byte(mac), []byte(unsubscribeMAC(secret, id))) {
		return 0, ErrInvalidUnsubscribeToken
	}
	userID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return 0, ErrInvalidUnsubscribeToken
	}
	return userID, nil
}

func unsubscribeMAC(secret, id string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte("unsubscribe:" + id)) // domain-separated so it can't be replayed elsewhere
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
