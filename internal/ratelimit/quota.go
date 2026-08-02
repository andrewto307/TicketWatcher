// Package ratelimit gates all outbound Ticketmaster traffic so the service stays
// within the free tier's 5 req/s and 5,000/day budget. See plan/05-rate-limiting.md.
package ratelimit

import (
	"errors"
	"sync"
	"time"
)

// Class distinguishes traffic so interactive search and background polling get
// separate slices of the daily budget and can't starve each other.
type Class int

const (
	ClassPoll Class = iota
	ClassSearch
)

func (c Class) String() string {
	switch c {
	case ClassPoll:
		return "poll"
	case ClassSearch:
		return "search"
	default:
		return "unknown"
	}
}

// ErrDailyBudgetExhausted is returned when a class has used its whole daily allowance.
var ErrDailyBudgetExhausted = errors.New("daily API budget exhausted for this class")

// DailyQuota tracks per-class usage against a daily budget, resetting at local midnight.
type DailyQuota struct {
	mu      sync.Mutex
	now     func() time.Time
	resetAt time.Time
	used    map[Class]int
	budget  map[Class]int
}

// NewDailyQuota builds a quota with the given per-class daily budgets.
func NewDailyQuota(pollBudget, searchBudget int) *DailyQuota {
	return newDailyQuotaWithClock(pollBudget, searchBudget, time.Now)
}

// newDailyQuotaWithClock lets tests inject a clock.
func newDailyQuotaWithClock(pollBudget, searchBudget int, now func() time.Time) *DailyQuota {
	return &DailyQuota{
		now:     now,
		resetAt: nextMidnight(now()),
		used:    map[Class]int{},
		budget:  map[Class]int{ClassPoll: pollBudget, ClassSearch: searchBudget},
	}
}

// Reserve claims one call for the class, or returns ErrDailyBudgetExhausted.
func (q *DailyQuota) Reserve(c Class) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.maybeReset()
	if q.used[c] >= q.budget[c] {
		return ErrDailyBudgetExhausted
	}
	q.used[c]++
	return nil
}

// Release refunds a previously reserved call (used when the call never happens).
func (q *DailyQuota) Release(c Class) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.used[c] > 0 {
		q.used[c]--
	}
}

// Remaining reports how many calls the class has left today.
func (q *DailyQuota) Remaining(c Class) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.maybeReset()
	if r := q.budget[c] - q.used[c]; r > 0 {
		return r
	}
	return 0
}

// Snapshot returns copies of usage/budget for observability (Phase 5 dashboard).
func (q *DailyQuota) Snapshot() (used, budget map[Class]int, resetAt time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.maybeReset()
	used, budget = map[Class]int{}, map[Class]int{}
	for k, v := range q.used {
		used[k] = v
	}
	for k, v := range q.budget {
		budget[k] = v
	}
	return used, budget, q.resetAt
}

func (q *DailyQuota) maybeReset() {
	if !q.now().Before(q.resetAt) {
		q.used = map[Class]int{}
		q.resetAt = nextMidnight(q.now())
	}
}

func nextMidnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, t.Location())
}
