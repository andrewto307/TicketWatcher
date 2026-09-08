package auth

import "testing"

func TestNewOpaqueToken_UniqueAndHashed(t *testing.T) {
	tok1, hash1, err := NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	tok2, hash2, err := NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}

	if tok1 == "" || hash1 == "" {
		t.Fatal("empty token or hash")
	}
	if tok1 == tok2 {
		t.Error("two tokens were identical — they must be unpredictable")
	}
	if hash1 == hash2 {
		t.Error("two hashes were identical")
	}
	// The stored form must not reveal the emailed value.
	if hash1 == tok1 {
		t.Error("hash equals the raw token — a DB leak would be directly replayable")
	}
	if got := HashToken(tok1); got != hash1 {
		t.Errorf("HashToken(token) = %q, want the hash returned at mint time (%q)", got, hash1)
	}
}

func TestHashToken_IsDeterministic(t *testing.T) {
	if HashToken("abc") != HashToken("abc") {
		t.Error("hashing is not deterministic — lookups by hash would fail")
	}
	if HashToken("abc") == HashToken("abd") {
		t.Error("different tokens hashed to the same value")
	}
}
