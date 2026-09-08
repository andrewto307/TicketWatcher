package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPLimiter throttles abusable public endpoints per client.
//
// It is the inbound counterpart to Limiter: that one protects the Ticketmaster
// budget on the way out, this one protects the auth endpoints on the way in
// (password brute-force, signup spam). Each key — a client IP — gets its own
// token bucket.
//
// State is in-memory, which matches the single-instance deployment constraint
// (see plan/07-deployment.md §1). If the app is ever scaled out, this moves to a
// shared store alongside the outbound limiter.
type IPLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*ipBucket
	limit     rate.Limit
	burst     int
	ttl       time.Duration // idle time after which a bucket is evicted
	lastSweep time.Time

	now func() time.Time // injectable for tests
}

type ipBucket struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// NewIPLimiter allows perMinute sustained requests per key, tolerating a burst of
// burst requests. Idle buckets are evicted after 10 minutes so the map can't grow
// without bound — an attacker rotating source IPs must not be able to exhaust
// memory through the limiter meant to stop them.
func NewIPLimiter(perMinute float64, burst int) *IPLimiter {
	if burst < 1 {
		burst = 1
	}
	return &IPLimiter{
		buckets: make(map[string]*ipBucket),
		limit:   rate.Limit(perMinute / 60), // per-minute -> per-second
		burst:   burst,
		ttl:     10 * time.Minute,
		now:     time.Now,
	}
}

// Allow reports whether the key may proceed. When it may not, the second return
// value is how long until a token frees up (for a Retry-After header).
func (l *IPLimiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweepLocked(now)

	b, ok := l.buckets[key]
	if !ok {
		b = &ipBucket{lim: rate.NewLimiter(l.limit, l.burst)}
		l.buckets[key] = b
	}
	b.lastSeen = now

	// Reserve-then-inspect rather than Allow, so a rejected caller can be told how
	// long to wait. Cancel puts the token back when we decide not to proceed.
	res := b.lim.ReserveN(now, 1)
	if !res.OK() {
		return false, time.Second
	}
	if d := res.DelayFrom(now); d > 0 {
		res.CancelAt(now)
		return false, d
	}
	return true, 0
}

// sweepLocked drops buckets untouched for longer than ttl. Amortized: it runs at
// most once per ttl, not on every request. Caller must hold the lock.
func (l *IPLimiter) sweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < l.ttl {
		return
	}
	l.lastSweep = now
	for k, b := range l.buckets {
		if now.Sub(b.lastSeen) > l.ttl {
			delete(l.buckets, k)
		}
	}
}

// Len reports how many buckets are currently tracked (used by tests).
func (l *IPLimiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
