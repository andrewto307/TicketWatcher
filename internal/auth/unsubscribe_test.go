package auth

import (
	"strings"
	"testing"
)

func TestUnsubscribeToken_RoundTrips(t *testing.T) {
	const secret = "s3cret"
	tok := NewUnsubscribeToken(secret, 42)

	got, err := ParseUnsubscribeToken(secret, tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != 42 {
		t.Errorf("user id = %d, want 42", got)
	}
}

// The link lives in every email forever, so the same token must keep working —
// unlike the single-use verification and reset tokens.
func TestUnsubscribeToken_IsStableAndReusable(t *testing.T) {
	const secret = "s3cret"
	if NewUnsubscribeToken(secret, 7) != NewUnsubscribeToken(secret, 7) {
		t.Error("token is not stable across calls — an old email's link would stop working")
	}
	tok := NewUnsubscribeToken(secret, 7)
	for i := 0; i < 3; i++ {
		if _, err := ParseUnsubscribeToken(secret, tok); err != nil {
			t.Fatalf("parse attempt %d failed: %v — the link must be reusable", i+1, err)
		}
	}
}

func TestUnsubscribeToken_RejectsForgery(t *testing.T) {
	const secret = "s3cret"
	valid := NewUnsubscribeToken(secret, 42)

	tests := []struct {
		name, token string
	}{
		{"empty", ""},
		{"no separator", "42"},
		{"tampered user id", "43." + strings.SplitN(valid, ".", 2)[1]},
		{"tampered mac", strings.SplitN(valid, ".", 2)[0] + ".AAAA"},
		{"garbage", "not.atoken"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseUnsubscribeToken(secret, tt.token); err == nil {
				t.Errorf("accepted %q, want rejection", tt.token)
			}
		})
	}

	// A token minted with a different secret must not verify here.
	if _, err := ParseUnsubscribeToken("other-secret", valid); err == nil {
		t.Error("token from a different secret was accepted")
	}
}

// Swapping the user id must change the MAC, or one user could opt out another.
func TestUnsubscribeToken_DistinctPerUser(t *testing.T) {
	const secret = "s3cret"
	if NewUnsubscribeToken(secret, 1) == NewUnsubscribeToken(secret, 2) {
		t.Fatal("two users produced the same token")
	}
}
