//go:build integration

package inttest

import (
	"context"
	"testing"
	"time"

	"ticket-watcher/internal/store"
	"ticket-watcher/internal/store/db"
)

// The exactly-once guarantee for milestone alerts rests on a real Postgres
// primary key, so it's worth asserting against the database rather than a fake.
func TestDB_ClaimWatchAlertIsExactlyOnce(t *testing.T) {
	q := setupDB(t)
	ctx := context.Background()

	user, err := q.GetUserByEmail(ctx, "demo@example.com")
	if err != nil {
		t.Fatalf("seeded demo user missing: %v", err)
	}
	ev, err := q.UpsertEventByTMID(ctx, db.UpsertEventByTMIDParams{
		TmEventID: "TMCLAIM", Name: "Claim Test", Url: "http://tm/claim",
	})
	if err != nil {
		t.Fatal(err)
	}
	w, err := q.CreateWatch(ctx, db.CreateWatchParams{
		UserID: user.ID, EventID: ev.ID, ConditionType: "becomes_available", PollIntervalS: 300,
	})
	if err != nil {
		t.Fatal(err)
	}

	claim := func(kind string) int64 {
		t.Helper()
		n, err := q.ClaimWatchAlert(ctx, db.ClaimWatchAlertParams{WatchID: w.ID, Kind: kind})
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	if got := claim("public_open"); got != 1 {
		t.Errorf("first claim returned %d rows, want 1 (the send should happen)", got)
	}
	if got := claim("public_open"); got != 0 {
		t.Errorf("second claim returned %d rows, want 0 (no duplicate email)", got)
	}
	// A different milestone on the same watch is independent.
	if got := claim("presale_open"); got != 1 {
		t.Errorf("different kind returned %d rows, want 1", got)
	}

	kinds, err := q.ListFiredAlertKinds(ctx, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(kinds) != 2 {
		t.Errorf("fired kinds = %v, want 2 distinct", kinds)
	}

	// Deleting the watch must cascade, or a re-created watch would inherit stale
	// "already told them" rows and stay permanently silent.
	if _, err := q.DeleteWatch(ctx, db.DeleteWatchParams{ID: w.ID, UserID: user.ID}); err != nil {
		t.Fatal(err)
	}
	after, err := q.ListFiredAlertKinds(ctx, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 0 {
		t.Errorf("watch_alerts survived watch deletion: %v", after)
	}
}

// Closing an event's watches is what stops the scheduler spending API budget on
// events that can never change again.
func TestDB_CloseWatchesForEventStopsPolling(t *testing.T) {
	q := setupDB(t)
	ctx := context.Background()

	user, err := q.GetUserByEmail(ctx, "demo@example.com")
	if err != nil {
		t.Fatal(err)
	}
	ev, err := q.UpsertEventByTMID(ctx, db.UpsertEventByTMIDParams{
		TmEventID: "TMCLOSE", Name: "Close Test", Url: "http://tm/close",
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := q.CreateWatch(ctx, db.CreateWatchParams{
			UserID: user.ID, EventID: ev.ID, ConditionType: "becomes_available", PollIntervalS: 300,
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Due now, and visible to the scheduler.
	if err := q.MarkEventDue(ctx, ev.ID); err != nil {
		t.Fatal(err)
	}
	due, err := q.ListDueEvents(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if !containsEvent(due, ev.ID) {
		t.Fatal("event with active watches is not due — scheduler would never poll it")
	}

	n, err := q.CloseWatchesForEvent(ctx, ev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("closed %d watches, want 2", n)
	}

	// With no active watches, ListDueEvents must skip it entirely.
	due, err = q.ListDueEvents(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if containsEvent(due, ev.ID) {
		t.Error("event is still due after closing its watches — polling would continue forever")
	}
}

// The sale calendar must round-trip, including the distinction between "no date"
// and "date not announced" (onsale_tbd).
func TestDB_SaleCalendarRoundTrips(t *testing.T) {
	q := setupDB(t)
	ctx := context.Background()

	ev, err := q.UpsertEventByTMID(ctx, db.UpsertEventByTMIDParams{
		TmEventID: "TMSALES", Name: "Sales Test", Url: "http://tm/sales",
	})
	if err != nil {
		t.Fatal(err)
	}

	onsale := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	end := onsale.Add(30 * 24 * time.Hour)
	presale := onsale.Add(-24 * time.Hour)
	name := "Artist Presale"
	avail := "offsale"

	if err := q.UpdateEventLatest(ctx, db.UpdateEventLatestParams{
		ID:                  ev.ID,
		LastAvailability:    &avail,
		PublicOnsaleAt:      store.TS(onsale),
		PublicOnsaleEndAt:   store.TS(end),
		EarliestPresaleAt:   store.TS(presale),
		EarliestPresaleName: &name,
		PresaleCount:        4,
		OnsaleTbd:           false,
		NextPollAt:          store.TS(time.Now().Add(5 * time.Minute)),
	}); err != nil {
		t.Fatal(err)
	}

	got, err := q.GetEvent(ctx, ev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.PublicOnsaleAt.Valid || !got.PublicOnsaleAt.Time.UTC().Equal(onsale) {
		t.Errorf("public_onsale_at = %v, want %v", got.PublicOnsaleAt.Time, onsale)
	}
	if !got.EarliestPresaleAt.Valid || !got.EarliestPresaleAt.Time.UTC().Equal(presale) {
		t.Errorf("earliest_presale_at = %v, want %v", got.EarliestPresaleAt.Time, presale)
	}
	if got.EarliestPresaleName == nil || *got.EarliestPresaleName != name {
		t.Errorf("earliest_presale_name = %v, want %q", got.EarliestPresaleName, name)
	}
	if got.PresaleCount != 4 {
		t.Errorf("presale_count = %d, want 4", got.PresaleCount)
	}
	if got.OnsaleTbd {
		t.Error("onsale_tbd = true, want false")
	}

	// Now the unannounced-date case: NULL date but the TBD flag set, which is a
	// different state from "we have no information".
	if err := q.UpdateEventLatest(ctx, db.UpdateEventLatestParams{
		ID:               ev.ID,
		LastAvailability: &avail,
		PresaleCount:     0,
		OnsaleTbd:        true,
		NextPollAt:       store.TS(time.Now().Add(5 * time.Minute)),
	}); err != nil {
		t.Fatal(err)
	}
	got, err = q.GetEvent(ctx, ev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PublicOnsaleAt.Valid {
		t.Error("public_onsale_at should be NULL when the date is unannounced")
	}
	if !got.OnsaleTbd {
		t.Error("onsale_tbd = false, want true")
	}
}

func containsEvent(evs []db.Event, id int64) bool {
	for _, e := range evs {
		if e.ID == id {
			return true
		}
	}
	return false
}
