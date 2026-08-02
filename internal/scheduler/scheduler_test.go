package scheduler

import (
	"context"
	"testing"
	"time"

	"ticket-watcher/internal/ratelimit"
	"ticket-watcher/internal/store/db"
)

type fakeDueLister struct {
	events    []db.Event
	lastLimit int32
}

func (f *fakeDueLister) ListDueEvents(_ context.Context, limit int32) ([]db.Event, error) {
	f.lastLimit = limit
	if int(limit) < len(f.events) {
		return f.events[:limit], nil
	}
	return f.events, nil
}

func makeEvents(n int) []db.Event {
	evs := make([]db.Event, n)
	for i := range evs {
		evs[i] = db.Event{ID: int64(i + 1)}
	}
	return evs
}

func drain(ch chan int64) []int64 {
	var out []int64
	for {
		select {
		case v := <-ch:
			out = append(out, v)
		default:
			return out
		}
	}
}

func TestScheduler_TickEnqueuesDueEvents(t *testing.T) {
	fake := &fakeDueLister{events: makeEvents(3)}
	quota := ratelimit.NewDailyQuota(1000, 0)
	jobs := make(chan int64, 10)
	s := New(fake, quota, jobs, time.Minute, 20)

	s.tick(context.Background())

	if got := drain(jobs); len(got) != 3 {
		t.Fatalf("enqueued %d events, want 3", len(got))
	}
}

func TestScheduler_TickCappedByBudget(t *testing.T) {
	fake := &fakeDueLister{events: makeEvents(5)}
	quota := ratelimit.NewDailyQuota(2, 0) // only 2 poll calls left today
	jobs := make(chan int64, 10)
	s := New(fake, quota, jobs, time.Minute, 20)

	s.tick(context.Background())

	if fake.lastLimit != 2 {
		t.Errorf("ListDueEvents limit = %d, want 2 (budget-capped)", fake.lastLimit)
	}
	if got := drain(jobs); len(got) != 2 {
		t.Errorf("enqueued %d, want 2", len(got))
	}
}

func TestScheduler_TickCappedByMaxTick(t *testing.T) {
	fake := &fakeDueLister{events: makeEvents(50)}
	quota := ratelimit.NewDailyQuota(1000, 0)
	jobs := make(chan int64, 100)
	s := New(fake, quota, jobs, time.Minute, 5) // maxTick = 5

	s.tick(context.Background())

	if fake.lastLimit != 5 {
		t.Errorf("ListDueEvents limit = %d, want 5 (maxTick-capped)", fake.lastLimit)
	}
}
