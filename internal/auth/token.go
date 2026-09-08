package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// opaqueTokenBytes is the entropy behind an emailed link. 32 bytes puts brute
// force far out of reach, which matters because these tokens are bearer
// credentials: whoever holds one can verify an address or reset a password.
const opaqueTokenBytes = 32

// NewOpaqueToken mints a random URL-safe token and returns it alongside the hash
// to persist. Only the hash is stored, so a database leak yields nothing
// replayable — the raw token exists solely in the email we send.
func NewOpaqueToken() (token, hash string, err error) {
	b := make([]byte, opaqueTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken returns the storage form of an opaque token.
//
// SHA-256 (not bcrypt) is deliberate: unlike a password, this token is 32 bytes
// of full-entropy randomness, so there is no dictionary to slow down — only a
// lookup key that must not be reversible.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
