//go:build integration

package inttest

import (
	"context"
	"testing"
	"time"

	"ticket-watcher/internal/service"
	"ticket-watcher/internal/store"
	"ticket-watcher/internal/store/db"
)

// Creating a watch must make its event due for polling immediately, so the new
// watch is evaluated on the next scheduler tick rather than waiting up to
// poll_interval_s for the event's next scheduled poll.
func TestWatchService_CreateMarksEventDue(t *testing.T) {
	q := setupDB(t)
	ctx := context.Background()
	user, err := q.GetUserByEmail(ctx, "demo@example.com")
	if err != nil {
		t.Fatalf("user: %v", err)
	}

	// An already-tracked event whose next poll is far in the future.
	ev, err := q.UpsertEventByTMID(ctx, db.UpsertEventByTMIDParams{TmEventID: "TMDUE", Name: "Show"})
	if err != nil {
		t.Fatalf("upsert event: %v", err)
	}
	if err := q.UpdateEventLatest(ctx, db.UpdateEventLatestParams{
		ID:         ev.ID,
		NextPollAt: store.TS(time.Now().Add(time.Hour)),
	}); err != nil {
		t.Fatalf("push next_poll_at into the future: %v", err)
	}

	// nil Ticketmaster client is safe here: the event already exists, so Create
	// resolves it from the DB and never calls the API.
	svc := service.NewWatchService(q, nil)
	if _, err := svc.Create(ctx, user.ID, service.CreateWatchInput{TMEventID: "TMDUE", ConditionType: "becomes_available"}); err != nil {
		t.Fatalf("create watch: %v", err)
	}

	got, err := q.GetEvent(ctx, ev.ID)
	if err != nil {
		t.Fatalf("get event: %v", err)
	}
	if got.NextPollAt.Time.After(time.Now()) {
		t.Errorf("event should be due now after a watch was created, but next_poll_at=%v is still in the future", got.NextPollAt.Time)
	}
}
