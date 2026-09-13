// Package worker runs the pool of goroutines that poll events off the jobs channel,
// write availability snapshots on change, evaluate each event's sale milestones, and
// fire edge-triggered alerts.
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
	// ClaimWatchAlert returns 1 the first time a (watch, kind) pair is claimed and
	// 0 afterwards, which is what makes each milestone alert fire exactly once.
	ClaimWatchAlert(ctx context.Context, arg db.ClaimWatchAlertParams) (int64, error)
	CloseWatchesForEvent(ctx context.Context, eventID int64) (int64, error)
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

// ProcessEvent polls one event, writes a snapshot if availability changed,
// reschedules it, then evaluates its active watches and fires alerts for any
// sale milestone the user hasn't been told about yet.
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

	// Capture whether we already knew the onsale date *before* overwriting it —
	// that false -> true transition is what makes an announcement detectable.
	knewPublicStart := ev.PublicOnsaleAt.Valid

	if err := d.Store.UpdateEventLatest(ctx, db.UpdateEventLatestParams{
		ID:                  eventID,
		LastAvailability:    &avail,
		PublicOnsaleAt:      store.TSPtr(snap.PublicOnsaleStart),
		PublicOnsaleEndAt:   store.TSPtr(snap.PublicOnsaleEnd),
		EarliestPresaleAt:   store.TSPtr(snap.EarliestPresaleStart),
		EarliestPresaleName: strPtr(snap.EarliestPresaleName),
		PresaleCount:        int32(snap.PresaleCount),
		OnsaleTbd:           snap.OnsaleTBD,
		NextPollAt:          store.TS(next),
	}); err != nil {
		return fmt.Errorf("update event: %w", err)
	}

	obs := evaluator.Observation{
		Now:             now(),
		Availability:    avail,
		PublicStart:     snap.PublicOnsaleStart,
		PublicEnd:       snap.PublicOnsaleEnd,
		OnsaleTBD:       snap.OnsaleTBD,
		EarliestPresale: snap.EarliestPresaleStart,
		EventDate:       store.TimePtr(ev.EventDate),
	}

	return evaluateWatches(ctx, ev, snap, obs, knewPublicStart, d)
}

// evaluateWatches runs the milestone rules for each active watch and sends the
// alerts that haven't fired yet.
func evaluateWatches(
	ctx context.Context,
	ev db.Event,
	snap ticketmaster.EventSnapshot,
	obs evaluator.Observation,
	knewPublicStart bool,
	d Deps,
) error {
	watches, err := d.Store.ListActiveWatchesForEvent(ctx, ev.ID)
	if err != nil {
		return fmt.Errorf("list watches: %w", err)
	}

	closeEvent := false
	for _, w := range watches {
		res := evaluator.Evaluate(obs, evaluator.Prior{
			KnewPublicStart: knewPublicStart,
			LastEvaluation:  w.LastEvaluation,
		})
		if res.CloseWatch {
			closeEvent = true
		}

		for _, kind := range res.Kinds {
			// Most kinds are level conditions: the evaluator reports them on every
			// poll while they hold, so the claim table is what makes them fire once.
			// The insert *is* the lock — if another poller already sent this
			// milestone we get 0 rows and stay quiet, with no read-then-write race.
			if needsClaim(kind) {
				claimed, err := d.Store.ClaimWatchAlert(ctx, db.ClaimWatchAlertParams{
					WatchID: w.ID, Kind: string(kind),
				})
				if err != nil {
					return fmt.Errorf("claim alert %s: %w", kind, err)
				}
				if claimed == 0 {
					continue // already told this user about this milestone
				}
			}

			// Consent gates. Checked after the claim so a withheld alert isn't
			// re-attempted on every poll for the rest of the event's life.
			switch {
			case !w.EmailVerifiedAt.Valid:
				log.Printf("worker: watch %d %s but %s is unverified — alert withheld", w.ID, kind, w.UserEmail)
				continue
			case w.UnsubscribedAt.Valid:
				log.Printf("worker: watch %d %s but %s has unsubscribed — alert withheld", w.ID, kind, w.UserEmail)
				continue
			}

			if d.Notifier != nil {
				var unsubURL string
				if d.UnsubscribeURL != nil {
					unsubURL = d.UnsubscribeURL(w.UserID)
				}
				d.Notifier.Notify(ctx, notifier.Alert{
					WatchID:              w.ID,
					ToEmail:              w.UserEmail,
					EventName:            ev.Name,
					Venue:                ev.Venue,
					EventURL:             ev.Url,
					Kind:                 string(kind),
					Availability:         obs.Availability,
					PublicOnsaleStart:    snap.PublicOnsaleStart,
					EarliestPresaleStart: snap.EarliestPresaleStart,
					EarliestPresaleName:  snap.EarliestPresaleName,
					OnsaleTBD:            snap.OnsaleTBD,
					UnsubscribeURL:       unsubURL,
				})
			}
			if err := d.Store.MarkNotified(ctx, w.ID); err != nil {
				return fmt.Errorf("mark notified: %w", err)
			}
		}

		if err := d.Store.SetLastEvaluation(ctx, db.SetLastEvaluationParams{
			ID: w.ID, LastEvaluation: res.StatusOnsale,
		}); err != nil {
			return fmt.Errorf("set last evaluation: %w", err)
		}
	}

	// The event is over (or cancelled): pause its watches so the scheduler stops
	// spending API budget on something that can never change again.
	if closeEvent {
		n, err := d.Store.CloseWatchesForEvent(ctx, ev.ID)
		if err != nil {
			return fmt.Errorf("close watches: %w", err)
		}
		if n > 0 {
			log.Printf("worker: event %d is over or cancelled — paused %d watch(es), polling stops", ev.ID, n)
		}
	}
	return nil
}

// needsClaim reports whether a kind must be deduplicated through watch_alerts.
//
// KindStatusOnsale is exempt: the evaluator only emits it on a rising
// offsale -> onsale edge (gated by last_evaluation), so it is already once-per-
// transition. Claiming it as well would permanently suppress the *next* edge —
// an event that sells out and later re-opens would never alert again, which is
// exactly the case the edge-trigger design exists to catch (06 D2).
//
// Every other kind is a level condition that holds across many polls, so the
// claim table is what turns it into a single alert.
func needsClaim(k evaluator.Kind) bool {
	return k != evaluator.KindStatusOnsale
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

// strPtr returns nil for an empty string so the column stays NULL rather than
// holding a meaningless "".
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
