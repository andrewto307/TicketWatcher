//go:build integration

package inttest

import (
	"context"
	"testing"

	"ticket-watcher/internal/store/db"
)

func strptr(s string) *string { return &s }

// TestDB_QueryRoundTrips exercises the full Phase 2/3 query set against real
// Postgres: seeded user, event upsert+dedupe, watch, snapshot, due-events,
// active-watch join (email), and the evaluation/notification writes.
func TestDB_QueryRoundTrips(t *testing.T) {
	q := setupDB(t)
	ctx := context.Background()

	user, err := q.GetUserByEmail(ctx, "demo@example.com")
	if err != nil {
		t.Fatalf("seeded demo user missing: %v", err)
	}

	ev, err := q.UpsertEventByTMID(ctx, db.UpsertEventByTMIDParams{
		TmEventID: "TM123", Name: "Test Show", Url: "http://x", Venue: "Arena",
	})
	if err != nil {
		t.Fatalf("upsert event: %v", err)
	}
	// Upserting the same tm_event_id must return the SAME row (per-event dedupe).
	ev2, err := q.UpsertEventByTMID(ctx, db.UpsertEventByTMIDParams{
		TmEventID: "TM123", Name: "Test Show v2", Url: "http://x", Venue: "Arena",
	})
	if err != nil || ev2.ID != ev.ID {
		t.Fatalf("upsert dedupe: ev=%d ev2=%d err=%v", ev.ID, ev2.ID, err)
	}

	w, err := q.CreateWatch(ctx, db.CreateWatchParams{
		UserID: user.ID, EventID: ev.ID, ConditionType: "becomes_available", PollIntervalS: 300,
	})
	if err != nil {
		t.Fatalf("create watch: %v", err)
	}

	if _, err := q.InsertAvailabilitySnapshot(ctx, db.InsertAvailabilitySnapshotParams{
		EventID: ev.ID, AvailabilityStatus: strptr("onsale"),
	}); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}

	if due, err := q.ListDueEvents(ctx, 10); err != nil || len(due) != 1 {
		t.Fatalf("list due events = %d (err %v), want 1", len(due), err)
	}

	aw, err := q.ListActiveWatchesForEvent(ctx, ev.ID)
	if err != nil || len(aw) != 1 || aw[0].UserEmail != "demo@example.com" {
		t.Fatalf("active watches = %+v (err %v), want 1 with demo email", aw, err)
	}

	lw, err := q.ListWatchesWithEvent(ctx, user.ID)
	if err != nil || len(lw) != 1 || lw[0].EventName != "Test Show v2" {
		t.Fatalf("watches-with-event = %+v (err %v)", lw, err)
	}

	if err := q.SetLastEvaluation(ctx, db.SetLastEvaluationParams{ID: w.ID, LastEvaluation: true}); err != nil {
		t.Fatalf("set last evaluation: %v", err)
	}
	if err := q.MarkNotified(ctx, w.ID); err != nil {
		t.Fatalf("mark notified: %v", err)
	}
	if _, err := q.InsertNotification(ctx, db.InsertNotificationParams{
		WatchID: w.ID, Channel: "email", Payload: []byte(`{"ok":true}`),
	}); err != nil {
		t.Fatalf("insert notification: %v", err)
	}
}
