package httpapi

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"

	"ticket-watcher/internal/ratelimit"
)

// rateLimit throttles a route group per client IP, answering 429 with Retry-After
// once a client exceeds its budget. Applied to the public auth endpoints, which
// are otherwise open to password brute-force and signup spam.
func rateLimit(l *ratelimit.IPLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, retryAfter := l.Allow(clientIP(r))
			if !ok {
				// Retry-After is whole seconds; never advertise 0 (that reads as
				// "retry immediately", which is exactly what we're preventing).
				secs := int(math.Ceil(retryAfter.Seconds()))
				if secs < 1 {
					secs = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(secs))
				writeError(w, http.StatusTooManyRequests, "too many requests — please slow down and try again shortly")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP resolves the caller's address. In production every request arrives
// through Fly's proxy, so RemoteAddr is the proxy and the real client is in a
// forwarding header. Trusting those headers is only safe *because* the app is
// never exposed directly — if that changes, this must be revisited, since a
// client can otherwise spoof X-Forwarded-For to dodge the limiter.
func clientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("Fly-Client-IP")); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Left-most entry is the original client; the rest are proxy hops.
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // no port (e.g. httptest) — use as-is
	}
	return host
}
