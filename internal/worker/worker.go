// Package worker runs the pool of goroutines that poll events off the jobs channel,
// write price snapshots on change, and reschedule each event.
package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"ticket-watcher/internal/money"
	"ticket-watcher/internal/store"
	"ticket-watcher/internal/store/db"
	"ticket-watcher/internal/ticketmaster"
)

// Store is the slice of the database the worker needs.
type Store interface {
	GetEvent(ctx context.Context, id int64) (db.Event, error)
	InsertPriceSnapshot(ctx context.Context, arg db.InsertPriceSnapshotParams) (db.PriceSnapshot, error)
	UpdateEventLatest(ctx context.Context, arg db.UpdateEventLatestParams) error
	MinPollIntervalForEvent(ctx context.Context, eventID int64) (int32, error)
}

// Fetcher is the slice of the Ticketmaster client the worker needs.
type Fetcher interface {
	GetEvent(ctx context.Context, tmEventID string) (ticketmaster.EventSnapshot, error)
}

// Deps are the collaborators a worker needs. Now is injectable for tests.
type Deps struct {
	Store Store
	TM    Fetcher
	Now   func() time.Time
}

// StartPool launches n worker goroutines draining jobs. Workers exit when jobs is
// closed (draining buffered work first); in-flight polls cancel via ctx.
func StartPool(ctx context.Context, n int, jobs <-chan int64, deps Deps, wg *sync.WaitGroup) {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for eventID := range jobs {
				if err := ProcessEvent(ctx, eventID, deps); err != nil {
					log.Printf("worker %d: event %d: %v", id, eventID, err)
				}
			}
		}(i)
	}
}

// ProcessEvent polls one event, writes a snapshot if anything changed, and
// reschedules it via next_poll_at. Condition evaluation/notification is Phase 3.
func ProcessEvent(ctx context.Context, eventID int64, d Deps) error {
	now := d.Now
	if now == nil {
		now = time.Now
	}

	ev, err := d.Store.GetEvent(ctx, eventID)
	if err != nil {
		return fmt.Errorf("get event: %w", err)
	}

	snap, err := d.TM.GetEvent(ctx, ev.TmEventID) // rate-limited; 429 backoff inside client
	if err != nil {
		// Budget exhausted / transient: skip, leave next_poll_at, retry later.
		return fmt.Errorf("poll: %w", err)
	}

	minC := money.ToCents(snap.MinPrice)
	maxC := money.ToCents(snap.MaxPrice)
	avail := snap.Availability

	if changed(ev, minC, maxC, avail) {
		if _, err := d.Store.InsertPriceSnapshot(ctx, db.InsertPriceSnapshotParams{
			EventID:            eventID,
			MinPriceCents:      minC,
			MaxPriceCents:      maxC,
			AvailabilityStatus: &avail,
		}); err != nil {
			return fmt.Errorf("insert snapshot: %w", err)
		}
	}

	interval, err := d.Store.MinPollIntervalForEvent(ctx, eventID)
	if err != nil || interval <= 0 {
		interval = 300
	}
	next := now().Add(time.Duration(interval) * time.Second)

	if err := d.Store.UpdateEventLatest(ctx, db.UpdateEventLatestParams{
		ID:                eventID,
		LastMinPriceCents: minC,
		LastMaxPriceCents: maxC,
		LastAvailability:  &avail,
		NextPollAt:        store.TS(next),
	}); err != nil {
		return fmt.Errorf("update event: %w", err)
	}
	return nil
}

// changed reports whether freshly polled values differ from the event's last state.
func changed(ev db.Event, minC, maxC *int64, avail string) bool {
	return !eqInt64Ptr(ev.LastMinPriceCents, minC) ||
		!eqInt64Ptr(ev.LastMaxPriceCents, maxC) ||
		!eqStrPtr(ev.LastAvailability, &avail)
}

func eqInt64Ptr(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func eqStrPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
