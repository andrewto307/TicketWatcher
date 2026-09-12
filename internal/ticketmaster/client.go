// Package ticketmaster is a thin, typed client over the Ticketmaster Discovery API.
// Every request is routed through a shared rate limiter (plan/05-rate-limiting.md)
// so total outbound traffic respects the 5 req/s and 5,000/day free-tier budget.
package ticketmaster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"ticket-watcher/internal/ratelimit"
)

// Client talks to the Ticketmaster Discovery API.
type Client struct {
	http    *http.Client
	baseURL string
	apiKey  string
	limiter *ratelimit.Limiter // may be nil (e.g. in tests) -> no limiting
}

// New constructs a Client. baseURL is e.g. https://app.ticketmaster.com/discovery/v2.
func New(baseURL, apiKey string, limiter *ratelimit.Limiter) *Client {
	return &Client{
		http:    &http.Client{Timeout: 10 * time.Second},
		baseURL: baseURL,
		apiKey:  apiKey,
		limiter: limiter,
	}
}

// Search returns events matching a keyword query (interactive traffic).
func (c *Client) Search(ctx context.Context, keyword string) ([]EventSnapshot, error) {
	q := url.Values{}
	q.Set("keyword", keyword)
	q.Set("apikey", c.apiKey)
	q.Set("size", "20")

	body, err := c.do(ctx, ratelimit.ClassSearch, fmt.Sprintf("%s/events.json?%s", c.baseURL, q.Encode()))
	if err != nil {
		return nil, err
	}
	return parseSearchResponse(body)
}

// GetEvent fetches a single event by its Ticketmaster id (background poll traffic).
func (c *Client) GetEvent(ctx context.Context, tmEventID string) (EventSnapshot, error) {
	q := url.Values{}
	q.Set("apikey", c.apiKey)

	body, err := c.do(ctx, ratelimit.ClassPoll, fmt.Sprintf("%s/events/%s.json?%s", c.baseURL, url.PathEscape(tmEventID), q.Encode()))
	if err != nil {
		return EventSnapshot{}, err
	}
	var e tmEvent
	if err := json.Unmarshal(body, &e); err != nil {
		return EventSnapshot{}, fmt.Errorf("decode event response: %w", err)
	}
	return e.toSnapshot(), nil
}

const maxRetries = 3

// do performs a rate-limited GET with 429 backoff. Each attempt acquires a fresh
// token so retries also respect the budget. The apikey is a query param, not a header.
func (c *Client) do(ctx context.Context, class ratelimit.Class, endpoint string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if c.limiter != nil {
			if err := c.limiter.Acquire(ctx, class); err != nil {
				return nil, err // ErrDailyBudgetExhausted or ctx cancelled
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			return body, nil
		case resp.StatusCode == http.StatusTooManyRequests:
			lastErr = fmt.Errorf("ticketmaster: 429 too many requests")
			wait := backoff(attempt)
			if d, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
				wait = d
			}
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		default:
			return nil, fmt.Errorf("ticketmaster: unexpected status %d: %s", resp.StatusCode, truncate(body, 200))
		}
	}
	return nil, fmt.Errorf("ticketmaster: retries exhausted: %w", lastErr)
}

// backoff returns an exponentially increasing delay with jitter: ~0.5s, 1s, 2s, 4s.
func backoff(attempt int) time.Duration {
	base := 500 * time.Millisecond
	return base*time.Duration(1<<attempt) + time.Duration(rand.Int63n(int64(base)))
}

func parseRetryAfter(v string) (time.Duration, bool) {
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second, true
	}
	return 0, false // (HTTP-date form not needed for our purposes)
}

// parseSearchResponse converts a raw Discovery search payload into snapshots.
// Split from the HTTP call so it can be unit-tested against fixtures.
func parseSearchResponse(data []byte) ([]EventSnapshot, error) {
	var sr searchResponse
	if err := json.Unmarshal(data, &sr); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	out := make([]EventSnapshot, 0, len(sr.Embedded.Events))
	for _, e := range sr.Embedded.Events {
		out = append(out, e.toSnapshot())
	}
	return out, nil
}

// toSnapshot maps a raw API event onto our normalized EventSnapshot, handling the
// optional fields gracefully (missing time, no venue).
func (e tmEvent) toSnapshot() EventSnapshot {
	s := EventSnapshot{
		TMEventID:    e.ID,
		Name:         e.Name,
		URL:          e.URL,
		Availability: normalizeStatus(e.Dates.Status.Code),
	}
	if len(e.Embedded.Venues) > 0 {
		s.Venue = e.Embedded.Venues[0].Name
	}
	if e.Dates.Start.DateTime != "" {
		if t, err := time.Parse(time.RFC3339, e.Dates.Start.DateTime); err == nil {
			s.EventDate = &t
		}
	}
	return s
}

func normalizeStatus(code string) string {
	if code == "" {
		return "unknown"
	}
	return code
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
