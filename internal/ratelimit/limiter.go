package ratelimit

import (
	"context"

	"golang.org/x/time/rate"
)

// Limiter is the single choke point for outbound Ticketmaster calls: a shared
// per-second token bucket plus a per-class daily quota.
type Limiter struct {
	bucket *rate.Limiter
	quota  *DailyQuota
}

// New builds a Limiter. perSecond/burst configure the token bucket (e.g. 5, 5).
func New(perSecond float64, burst int, quota *DailyQuota) *Limiter {
	return &Limiter{
		bucket: rate.NewLimiter(rate.Limit(perSecond), burst),
		quota:  quota,
	}
}

// Acquire blocks until the daily quota allows the call and a per-second token is
// free. It returns ErrDailyBudgetExhausted immediately if the class is out of budget,
// and refunds the reservation if the wait is cancelled (the call won't happen).
func (l *Limiter) Acquire(ctx context.Context, c Class) error {
	if err := l.quota.Reserve(c); err != nil {
		return err
	}
	if err := l.bucket.Wait(ctx); err != nil {
		l.quota.Release(c)
		return err
	}
	return nil
}

// Quota exposes the underlying daily quota (for the scheduler's budgeting and observability).
func (l *Limiter) Quota() *DailyQuota { return l.quota }
