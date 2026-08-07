package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPasswordHashing(t *testing.T) {
	h, err := HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "secret123") {
		t.Error("correct password should verify")
	}
	if CheckPassword(h, "wrong") {
		t.Error("wrong password should not verify")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	tok, err := NewToken("s3cr3t", 42, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	uid, err := ParseToken("s3cr3t", tok)
	if err != nil {
		t.Fatal(err)
	}
	if uid != 42 {
		t.Errorf("uid = %d, want 42", uid)
	}
}

func TestParseToken_WrongSecret(t *testing.T) {
	tok, _ := NewToken("right", 1, time.Hour)
	if _, err := ParseToken("wrong", tok); err == nil {
		t.Error("wrong secret should fail verification")
	}
}

func TestParseToken_Expired(t *testing.T) {
	tok, _ := NewToken("s", 1, -time.Hour) // already expired
	if _, err := ParseToken("s", tok); err == nil {
		t.Error("expired token should fail")
	}
}

func TestMiddleware(t *testing.T) {
	secret := "s"
	tok, _ := NewToken(secret, 7, time.Hour)

	var gotUID int64
	var gotOK bool
	h := Middleware(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUID, gotOK = UserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// valid token -> handler runs with the user id in context
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !gotOK || gotUID != 7 {
		t.Errorf("with token: code=%d uid=%d ok=%v", rec.Code, gotUID, gotOK)
	}

	// no token -> 401
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("no token: code=%d, want 401", rec2.Code)
	}
}
