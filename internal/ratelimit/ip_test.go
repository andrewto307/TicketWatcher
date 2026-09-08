package ratelimit

import (
	"testing"
	"time"
)

// A client may burst, then gets refused until the bucket refills.
func TestIPLimiter_BurstThenRefuse(t *testing.T) {
	l := NewIPLimiter(60, 3) // 60/min = 1/s, burst 3
	base := time.Now()
	l.now = func() time.Time { return base }

	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("1.2.3.4"); !ok {
			t.Fatalf("request %d denied, want allowed (within burst)", i+1)
		}
	}
	ok, retry := l.Allow("1.2.3.4")
	if ok {
		t.Fatal("4th request allowed, want denied (burst exhausted)")
	}
	if retry <= 0 {
		t.Errorf("retry-after = %v, want > 0 so the caller knows when to come back", retry)
	}
}

// One noisy client must not consume another client's budget.
func TestIPLimiter_IsolatesClients(t *testing.T) {
	l := NewIPLimiter(60, 1)
	base := time.Now()
	l.now = func() time.Time { return base }

	if ok, _ := l.Allow("1.1.1.1"); !ok {
		t.Fatal("first client denied")
	}
	if ok, _ := l.Allow("1.1.1.1"); ok {
		t.Fatal("first client got a second token, want denied")
	}
	if ok, _ := l.Allow("2.2.2.2"); !ok {
		t.Error("second client denied — buckets are not isolated per key")
	}
}

// Tokens come back as time passes.
func TestIPLimiter_RefillsOverTime(t *testing.T) {
	l := NewIPLimiter(60, 1) // 1 per second
	now := time.Now()
	l.now = func() time.Time { return now }

	if ok, _ := l.Allow("1.2.3.4"); !ok {
		t.Fatal("first request denied")
	}
	if ok, _ := l.Allow("1.2.3.4"); ok {
		t.Fatal("immediate second request allowed, want denied")
	}
	now = now.Add(2 * time.Second)
	if ok, _ := l.Allow("1.2.3.4"); !ok {
		t.Error("request after refill denied, want allowed")
	}
}

// Idle buckets are evicted: the limiter must not become a memory leak for an
// attacker who rotates source addresses.
func TestIPLimiter_EvictsIdleBuckets(t *testing.T) {
	l := NewIPLimiter(60, 1)
	now := time.Now()
	l.now = func() time.Time { return now }

	l.Allow("1.1.1.1")
	l.Allow("2.2.2.2")
	if got := l.Len(); got != 2 {
		t.Fatalf("tracked buckets = %d, want 2", got)
	}

	// Past the TTL, a later request sweeps the stale entries (leaving only itself).
	now = now.Add(11 * time.Minute)
	l.Allow("3.3.3.3")
	if got := l.Len(); got != 1 {
		t.Errorf("tracked buckets = %d, want 1 (idle entries should be evicted)", got)
	}
}
