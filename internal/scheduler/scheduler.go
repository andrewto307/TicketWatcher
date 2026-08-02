// Package scheduler periodically finds events that are due to be polled and
// enqueues them onto the jobs channel for the worker pool to consume.
package scheduler

import (
	"context"
	"log"
	"time"

	"ticket-watcher/internal/ratelimit"
	"ticket-watcher/internal/store/db"
)

// DueLister is the slice of the store the scheduler needs.
type DueLister interface {
	ListDueEvents(ctx context.Context, limit int32) ([]db.Event, error)
}

// Scheduler ticks on an interval, enqueuing due events (capped by the remaining
// daily poll budget and a per-tick cap so calls stagger instead of spiking).
type Scheduler struct {
	q        DueLister
	quota    *ratelimit.DailyQuota
	jobs     chan<- int64
	interval time.Duration
	maxTick  int
}

func New(q DueLister, quota *ratelimit.DailyQuota, jobs chan<- int64, interval time.Duration, maxTick int) *Scheduler {
	return &Scheduler{q: q, quota: quota, jobs: jobs, interval: interval, maxTick: maxTick}
}

// Run loops until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	t := time.NewTicker(s.interval)
	defer t.Stop()
	log.Printf("scheduler: running every %s (<= %d events/tick)", s.interval, s.maxTick)
	for {
		select {
		case <-ctx.Done():
			log.Print("scheduler: stopped")
			return
		case <-t.C:
			s.tick(ctx)
		}
	}
}

// tick enqueues due events, bounded by the remaining poll budget and maxTick.
func (s *Scheduler) tick(ctx context.Context) {
	budget := s.quota.Remaining(ratelimit.ClassPoll)
	if budget > s.maxTick {
		budget = s.maxTick
	}
	if budget <= 0 {
		return
	}
	events, err := s.q.ListDueEvents(ctx, int32(budget))
	if err != nil {
		log.Printf("scheduler: list due events: %v", err)
		return
	}
	for _, e := range events {
		select {
		case s.jobs <- e.ID:
		case <-ctx.Done():
			return
		default:
			// Workers are saturated; leave the rest for the next tick.
			return
		}
	}
}
