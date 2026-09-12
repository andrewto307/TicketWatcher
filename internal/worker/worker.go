// Package worker runs the pool of goroutines that poll events off the jobs channel,
// write availability snapshots on change, evaluate each event's watches, and fire
// edge-triggered alerts.
package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"ticket-watcher/internal/evaluator"
	"ticket-watcher/internal/notifier"
	"ticket-watcher/internal/store"
	"ticket-watcher/internal/store/db"
	"ticket-watcher/internal/ticketmaster"
)

// Store is the slice of the database the worker needs.
type Store interface {
	GetEvent(ctx context.Context, id int64) (db.Event, error)
	InsertAvailabilitySnapshot(ctx context.Context, arg db.InsertAvailabilitySnapshotParams) (db.AvailabilitySnapshot, error)
	UpdateEventLatest(ctx context.Context, arg db.UpdateEventLatestParams) error
	MinPollIntervalForEvent(ctx context.Context, eventID int64) (int32, error)
	ListActiveWatchesForEvent(ctx context.Context, eventID int64) ([]db.ListActiveWatchesForEventRow, error)
	SetLastEvaluation(ctx context.Context, arg db.SetLastEvaluationParams) error
	MarkNotified(ctx context.Context, id int64) error
}

// Fetcher is the slice of the Ticketmaster client the worker needs.
type Fetcher interface {
	GetEvent(ctx context.Context, tmEventID string) (ticketmaster.EventSnapshot, error)
}

// Notifier delivers a fired-condition alert.
type Notifier interface {
	Notify(ctx context.Context, a notifier.Alert)
}

// Deps are the collaborators a worker needs. Now is injectable for tests.
type Deps struct {
	Store    Store
	TM       Fetcher
	Notifier Notifier
	Now      func() time.Time
	// UnsubscribeURL builds a user's opt-out link. Injected as a closure so the
	// worker needs neither the app secret nor the public URL. Nil in tests.
	UnsubscribeURL func(userID int64) string
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

// ProcessEvent polls one event, writes a snapshot if anything changed, reschedules
// it, then evaluates its active watches and fires edge-triggered alerts.
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

	avail := snap.Availability

	// Snapshot on change only: a quiet event polled 288x/day would otherwise write
	// 288 identical rows. The result reads as a clean list of transitions, which is
	// exactly what matters here — "went on sale at 10:02" is the product.
	if changed(ev, avail) {
		if _, err := d.Store.InsertAvailabilitySnapshot(ctx, db.InsertAvailabilitySnapshotParams{
			EventID:            eventID,
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
		ID:               eventID,
		LastAvailability: &avail,
		NextPollAt:       store.TS(next),
	}); err != nil {
		return fmt.Errorf("update event: %w", err)
	}

	return evaluateWatches(ctx, ev, avail, d)
}

// evaluateWatches checks each active watch on the event and fires an alert on the
// rising edge (condition transitions false -> true). It always records the latest
// evaluation so the next poll can detect the next edge.
func evaluateWatches(ctx context.Context, ev db.Event, avail string, d Deps) error {
	watches, err := d.Store.ListActiveWatchesForEvent(ctx, ev.ID)
	if err != nil {
		return fmt.Errorf("list watches: %w", err)
	}
	for _, w := range watches {
		met := evaluator.Met(
			evaluator.Condition{Type: w.ConditionType},
			evaluator.Observation{Availability: avail},
		)

		// Rising edge (false -> true). Two consent checks gate delivery: the owner
		// must have confirmed the address, and must not have opted out. Either way
		// the evaluation below is still recorded, so the watch stays correctly
		// armed and nothing silently drifts out of sync.
		if met && !w.LastEvaluation {
			switch {
			case !w.EmailVerifiedAt.Valid:
				log.Printf("worker: watch %d fired but %s is unverified — alert withheld", w.ID, w.UserEmail)
			case w.UnsubscribedAt.Valid:
				log.Printf("worker: watch %d fired but %s has unsubscribed — alert withheld", w.ID, w.UserEmail)
			default:
				if d.Notifier != nil {
					var unsubURL string
					if d.UnsubscribeURL != nil {
						unsubURL = d.UnsubscribeURL(w.UserID)
					}
					d.Notifier.Notify(ctx, notifier.Alert{
						WatchID:        w.ID,
						ToEmail:        w.UserEmail,
						EventName:      ev.Name,
						Venue:          ev.Venue,
						EventURL:       ev.Url,
						ConditionType:  w.ConditionType,
						Availability:   avail,
						UnsubscribeURL: unsubURL,
					})
				}
				if err := d.Store.MarkNotified(ctx, w.ID); err != nil {
					return fmt.Errorf("mark notified: %w", err)
				}
			}
		}

		if err := d.Store.SetLastEvaluation(ctx, db.SetLastEvaluationParams{ID: w.ID, LastEvaluation: met}); err != nil {
			return fmt.Errorf("set last evaluation: %w", err)
		}
	}
	return nil
}

// changed reports whether the freshly polled availability differs from the
// event's last known state.
func changed(ev db.Event, avail string) bool {
	return !eqStrPtr(ev.LastAvailability, &avail)
}

func eqStrPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
