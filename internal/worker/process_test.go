package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"ticket-watcher/internal/notifier"
	"ticket-watcher/internal/store"
	"ticket-watcher/internal/store/db"
	"ticket-watcher/internal/ticketmaster"
)

// verified marks a fixture watch's owner as having confirmed their address —
// the precondition for any alert being sent.
func verified() pgtype.Timestamptz { return store.TS(time.Now()) }

// --- stateful fakes ---

type fakeStore struct {
	event     db.Event
	watch     db.ListActiveWatchesForEventRow
	snapshots int
	marked    int
}

func (f *fakeStore) GetEvent(context.Context, int64) (db.Event, error) { return f.event, nil }

func (f *fakeStore) InsertPriceSnapshot(context.Context, db.InsertPriceSnapshotParams) (db.PriceSnapshot, error) {
	f.snapshots++
	return db.PriceSnapshot{}, nil
}

func (f *fakeStore) UpdateEventLatest(_ context.Context, arg db.UpdateEventLatestParams) error {
	f.event.LastMinPriceCents = arg.LastMinPriceCents
	f.event.LastMaxPriceCents = arg.LastMaxPriceCents
	f.event.LastAvailability = arg.LastAvailability
	return nil
}

func (f *fakeStore) MinPollIntervalForEvent(context.Context, int64) (int32, error) { return 300, nil }

func (f *fakeStore) ListActiveWatchesForEvent(context.Context, int64) ([]db.ListActiveWatchesForEventRow, error) {
	return []db.ListActiveWatchesForEventRow{f.watch}, nil
}

func (f *fakeStore) SetLastEvaluation(_ context.Context, arg db.SetLastEvaluationParams) error {
	f.watch.LastEvaluation = arg.LastEvaluation // persist across polls
	return nil
}

func (f *fakeStore) MarkNotified(context.Context, int64) error { f.marked++; return nil }

type fakeFetcher struct {
	min   *float64
	avail string
}

func (f *fakeFetcher) GetEvent(context.Context, string) (ticketmaster.EventSnapshot, error) {
	return ticketmaster.EventSnapshot{TMEventID: "x", Name: "Test", MinPrice: f.min, Availability: f.avail}, nil
}

type fakeNotifier struct {
	calls int
	last  notifier.Alert
}

func (f *fakeNotifier) Notify(_ context.Context, a notifier.Alert) { f.calls++; f.last = a }

func usd(v float64) *float64 { return &v }

// TestProcessEvent_EdgeTriggered drives a full price sequence through ProcessEvent
// and asserts alerts fire only on the false->true edge — the crux of Phase 3.
func TestProcessEvent_EdgeTriggered(t *testing.T) {
	thr := int64(20000) // $200.00
	st := &fakeStore{
		event: db.Event{ID: 1, TmEventID: "x", Name: "Test"},
		watch: db.ListActiveWatchesForEventRow{
			ID: 1, ConditionType: "price_below", ThresholdCents: &thr, Status: "active",
			UserEmail: "u@e.com", EmailVerifiedAt: verified(),
		},
	}
	fetch := &fakeFetcher{avail: "onsale"}
	notif := &fakeNotifier{}
	deps := Deps{Store: st, TM: fetch, Notifier: notif, Now: time.Now}

	seq := []struct {
		price     float64
		wantCalls int // cumulative notifier calls expected after this poll
	}{
		{180, 1}, // false -> true : FIRE
		{190, 1}, // true  -> true : still below, no re-fire
		{210, 1}, // true  -> false: rose above threshold
		{150, 2}, // false -> true : FIRE again (re-armed)
	}
	for i, step := range seq {
		fetch.min = usd(step.price)
		if err := ProcessEvent(context.Background(), 1, deps); err != nil {
			t.Fatalf("poll %d ($%.0f): %v", i, step.price, err)
		}
		if notif.calls != step.wantCalls {
			t.Errorf("after $%.0f: notifier calls = %d, want %d", step.price, notif.calls, step.wantCalls)
		}
	}
	if st.marked != 2 {
		t.Errorf("MarkNotified called %d times, want 2", st.marked)
	}
	if notif.last.ToEmail != "u@e.com" {
		t.Errorf("alert addressed to %q, want u@e.com", notif.last.ToEmail)
	}
}

// TestProcessEvent_NeverTrue confirms a watch whose condition stays false never fires.
func TestProcessEvent_NeverTrue(t *testing.T) {
	thr := int64(1000) // $10.00 — well below the polled price
	st := &fakeStore{
		event: db.Event{ID: 1, TmEventID: "x", Name: "Test"},
		watch: db.ListActiveWatchesForEventRow{
			ID: 1, ConditionType: "price_below", ThresholdCents: &thr, Status: "active",
			UserEmail: "u@e.com", EmailVerifiedAt: verified(),
		},
	}
	fetch := &fakeFetcher{min: usd(288.69), avail: "onsale"}
	notif := &fakeNotifier{}
	deps := Deps{Store: st, TM: fetch, Notifier: notif, Now: time.Now}

	for i := 0; i < 3; i++ {
		if err := ProcessEvent(context.Background(), 1, deps); err != nil {
			t.Fatalf("poll %d: %v", i, err)
		}
	}
	if notif.calls != 0 {
		t.Errorf("notifier calls = %d, want 0 (condition never met)", notif.calls)
	}
}

// TestProcessEvent_UnsubscribedUserIsNotAlerted covers the Tier 3 rule: someone
// who opted out must stop receiving mail, even though their watch still fires.
func TestProcessEvent_UnsubscribedUserIsNotAlerted(t *testing.T) {
	thr := int64(20000) // $200.00
	st := &fakeStore{
		event: db.Event{ID: 1, TmEventID: "x", Name: "Test"},
		watch: db.ListActiveWatchesForEventRow{
			ID: 1, ConditionType: "price_below", ThresholdCents: &thr, Status: "active",
			UserEmail: "opted-out@e.com", EmailVerifiedAt: verified(), UnsubscribedAt: verified(),
		},
	}
	fetch := &fakeFetcher{min: usd(180), avail: "onsale"} // condition is met
	notif := &fakeNotifier{}
	deps := Deps{Store: st, TM: fetch, Notifier: notif, Now: time.Now}

	if err := ProcessEvent(context.Background(), 1, deps); err != nil {
		t.Fatal(err)
	}
	if notif.calls != 0 {
		t.Errorf("notifier calls = %d, want 0 (user unsubscribed)", notif.calls)
	}
	if st.marked != 0 {
		t.Errorf("MarkNotified called %d times, want 0 (nothing was sent)", st.marked)
	}
	if !st.watch.LastEvaluation {
		t.Error("last_evaluation = false, want true (the watch must still track the edge)")
	}
}

// Every alert must carry the opt-out link, or the unsubscribe requirement is
// only theoretically satisfied.
func TestProcessEvent_AlertCarriesUnsubscribeURL(t *testing.T) {
	thr := int64(20000)
	st := &fakeStore{
		event: db.Event{ID: 1, TmEventID: "x", Name: "Test"},
		watch: db.ListActiveWatchesForEventRow{
			ID: 1, UserID: 99, ConditionType: "price_below", ThresholdCents: &thr, Status: "active",
			UserEmail: "u@e.com", EmailVerifiedAt: verified(),
		},
	}
	fetch := &fakeFetcher{min: usd(180), avail: "onsale"}
	notif := &fakeNotifier{}
	deps := Deps{
		Store: st, TM: fetch, Notifier: notif, Now: time.Now,
		UnsubscribeURL: func(userID int64) string {
			return fmt.Sprintf("https://app.test/api/unsubscribe?token=u%d", userID)
		},
	}

	if err := ProcessEvent(context.Background(), 1, deps); err != nil {
		t.Fatal(err)
	}
	if notif.calls != 1 {
		t.Fatalf("notifier calls = %d, want 1", notif.calls)
	}
	if got, want := notif.last.UnsubscribeURL, "https://app.test/api/unsubscribe?token=u99"; got != want {
		t.Errorf("UnsubscribeURL = %q, want %q (built for the watch's owner)", got, want)
	}
}

// TestProcessEvent_UnverifiedEmailIsNotAlerted covers the Tier 1 rule: a met
// condition must not mail an address whose owner never confirmed it. The watch
// still tracks its evaluation, so it stays armed for after they verify.
func TestProcessEvent_UnverifiedEmailIsNotAlerted(t *testing.T) {
	thr := int64(20000) // $200.00
	st := &fakeStore{
		event: db.Event{ID: 1, TmEventID: "x", Name: "Test"},
		watch: db.ListActiveWatchesForEventRow{
			ID: 1, ConditionType: "price_below", ThresholdCents: &thr, Status: "active",
			UserEmail: "unverified@e.com", // EmailVerifiedAt left zero => NULL => unverified
		},
	}
	fetch := &fakeFetcher{min: usd(180), avail: "onsale"} // condition is met
	notif := &fakeNotifier{}
	deps := Deps{Store: st, TM: fetch, Notifier: notif, Now: time.Now}

	if err := ProcessEvent(context.Background(), 1, deps); err != nil {
		t.Fatal(err)
	}
	if notif.calls != 0 {
		t.Errorf("notifier calls = %d, want 0 (owner is unverified)", notif.calls)
	}
	if st.marked != 0 {
		t.Errorf("MarkNotified called %d times, want 0 (nothing was sent)", st.marked)
	}
	if !st.watch.LastEvaluation {
		t.Error("last_evaluation = false, want true (the watch must still track the edge)")
	}
}
