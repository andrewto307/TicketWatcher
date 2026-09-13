package worker

import (
	"context"
	"fmt"
	"sort"
	"strings"
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

// activeWatch is the common fixture: an active becomes_available watch whose
// owner is verified and subscribed, i.e. eligible for alerts.
func activeWatch() db.ListActiveWatchesForEventRow {
	return db.ListActiveWatchesForEventRow{
		ID: 1, UserID: 99, ConditionType: "becomes_available", Status: "active",
		UserEmail: "u@e.com", EmailVerifiedAt: verified(),
	}
}

// --- stateful fakes ---

type fakeStore struct {
	event     db.Event
	watch     db.ListActiveWatchesForEventRow
	snapshots int
	marked    int

	// claimed records which (watch, kind) alerts have been claimed, mirroring the
	// real table's PK so a second claim of the same kind returns 0 rows.
	claimed map[string]bool
	closed  int
}

func (f *fakeStore) ClaimWatchAlert(_ context.Context, arg db.ClaimWatchAlertParams) (int64, error) {
	if f.claimed == nil {
		f.claimed = map[string]bool{}
	}
	k := fmt.Sprintf("%d:%s", arg.WatchID, arg.Kind)
	if f.claimed[k] {
		return 0, nil
	}
	f.claimed[k] = true
	return 1, nil
}

func (f *fakeStore) CloseWatchesForEvent(_ context.Context, _ int64) (int64, error) {
	f.closed++
	return 1, nil
}

// kinds returns the alert kinds claimed for a watch, for assertions.
func (f *fakeStore) kinds(watchID int64) []string {
	var out []string
	prefix := fmt.Sprintf("%d:", watchID)
	for k := range f.claimed {
		if strings.HasPrefix(k, prefix) {
			out = append(out, strings.TrimPrefix(k, prefix))
		}
	}
	sort.Strings(out)
	return out
}

func (f *fakeStore) GetEvent(context.Context, int64) (db.Event, error) { return f.event, nil }

func (f *fakeStore) InsertAvailabilitySnapshot(context.Context, db.InsertAvailabilitySnapshotParams) (db.AvailabilitySnapshot, error) {
	f.snapshots++
	return db.AvailabilitySnapshot{}, nil
}

func (f *fakeStore) UpdateEventLatest(_ context.Context, arg db.UpdateEventLatestParams) error {
	f.event.LastAvailability = arg.LastAvailability
	f.event.PublicOnsaleAt = arg.PublicOnsaleAt
	f.event.PublicOnsaleEndAt = arg.PublicOnsaleEndAt
	f.event.EarliestPresaleAt = arg.EarliestPresaleAt
	f.event.OnsaleTbd = arg.OnsaleTbd
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
	avail       string
	publicStart *time.Time
	publicEnd   *time.Time
	presale     *time.Time
	presaleName string
	onsaleTBD   bool
}

func (f *fakeFetcher) GetEvent(context.Context, string) (ticketmaster.EventSnapshot, error) {
	return ticketmaster.EventSnapshot{
		TMEventID: "x", Name: "Test", Availability: f.avail,
		PublicOnsaleStart:    f.publicStart,
		PublicOnsaleEnd:      f.publicEnd,
		EarliestPresaleStart: f.presale,
		EarliestPresaleName:  f.presaleName,
		OnsaleTBD:            f.onsaleTBD,
	}, nil
}

type fakeNotifier struct {
	calls int
	last  notifier.Alert
}

func (f *fakeNotifier) Notify(_ context.Context, a notifier.Alert) { f.calls++; f.last = a }

// TestProcessEvent_EdgeTriggered drives an availability sequence through
// ProcessEvent and asserts alerts fire only on the false->true edge — the crux
// of Phase 3. An event that stays on sale must not re-alert every poll.
func TestProcessEvent_EdgeTriggered(t *testing.T) {
	st := &fakeStore{
		event: db.Event{ID: 1, TmEventID: "x", Name: "Test"},
		watch: activeWatch(),
	}
	fetch := &fakeFetcher{}
	notif := &fakeNotifier{}
	deps := Deps{Store: st, TM: fetch, Notifier: notif, Now: time.Now}

	seq := []struct {
		avail     string
		wantCalls int // cumulative notifier calls expected after this poll
	}{
		{"onsale", 1},  // false -> true : FIRE
		{"onsale", 1},  // true  -> true : still on sale, no re-fire
		{"offsale", 1}, // true  -> false: sale closed
		{"onsale", 2},  // false -> true : FIRE again (re-armed)
	}
	for i, step := range seq {
		fetch.avail = step.avail
		if err := ProcessEvent(context.Background(), 1, deps); err != nil {
			t.Fatalf("poll %d (%s): %v", i, step.avail, err)
		}
		if notif.calls != step.wantCalls {
			t.Errorf("after %s: notifier calls = %d, want %d", step.avail, notif.calls, step.wantCalls)
		}
	}
	if st.marked != 2 {
		t.Errorf("MarkNotified called %d times, want 2", st.marked)
	}
	if notif.last.ToEmail != "u@e.com" {
		t.Errorf("alert addressed to %q, want u@e.com", notif.last.ToEmail)
	}
}

// Snapshots are written only when availability changes, so a quiet event doesn't
// accumulate identical rows.
func TestProcessEvent_SnapshotsOnlyOnChange(t *testing.T) {
	st := &fakeStore{
		event: db.Event{ID: 1, TmEventID: "x", Name: "Test"},
		watch: activeWatch(),
	}
	fetch := &fakeFetcher{avail: "offsale"}
	deps := Deps{Store: st, TM: fetch, Now: time.Now}

	// First poll records the initial state; two more identical polls add nothing.
	for i := 0; i < 3; i++ {
		if err := ProcessEvent(context.Background(), 1, deps); err != nil {
			t.Fatal(err)
		}
	}
	if st.snapshots != 1 {
		t.Errorf("snapshots = %d after 3 identical polls, want 1", st.snapshots)
	}

	fetch.avail = "onsale" // a real transition
	if err := ProcessEvent(context.Background(), 1, deps); err != nil {
		t.Fatal(err)
	}
	if st.snapshots != 2 {
		t.Errorf("snapshots = %d after a change, want 2", st.snapshots)
	}
}

// TestProcessEvent_NeverTrue confirms a watch whose condition stays false never fires.
func TestProcessEvent_NeverTrue(t *testing.T) {
	st := &fakeStore{
		event: db.Event{ID: 1, TmEventID: "x", Name: "Test"},
		watch: activeWatch(),
	}
	fetch := &fakeFetcher{avail: "offsale"} // never on sale
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
	w := activeWatch()
	w.UserEmail, w.UnsubscribedAt = "opted-out@e.com", verified()
	st := &fakeStore{event: db.Event{ID: 1, TmEventID: "x", Name: "Test"}, watch: w}

	fetch := &fakeFetcher{avail: "onsale"} // condition is met
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

// TestProcessEvent_UnverifiedEmailIsNotAlerted covers the Tier 1 rule: a met
// condition must not mail an address whose owner never confirmed it. The watch
// still tracks its evaluation, so it stays armed for after they verify.
func TestProcessEvent_UnverifiedEmailIsNotAlerted(t *testing.T) {
	w := activeWatch()
	w.UserEmail = "unverified@e.com"
	w.EmailVerifiedAt = pgtype.Timestamptz{} // NULL => unverified
	st := &fakeStore{event: db.Event{ID: 1, TmEventID: "x", Name: "Test"}, watch: w}

	fetch := &fakeFetcher{avail: "onsale"} // condition is met
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

// Every alert must carry the opt-out link, or the unsubscribe requirement is
// only theoretically satisfied.
func TestProcessEvent_AlertCarriesUnsubscribeURL(t *testing.T) {
	st := &fakeStore{
		event: db.Event{ID: 1, TmEventID: "x", Name: "Test"},
		watch: activeWatch(),
	}
	fetch := &fakeFetcher{avail: "onsale"}
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

// --- sale-milestone behaviour ---

// A presale opening and the public sale opening are separate milestones and must
// produce separate alerts, not one merged notification.
func TestProcessEvent_PresaleAndPublicAreDistinctAlerts(t *testing.T) {
	base := time.Now()
	presale := base.Add(-2 * time.Hour)
	public := base.Add(2 * time.Hour)
	end := base.Add(30 * 24 * time.Hour)

	// The onsale date is already stored, so this test isolates presale vs public
	// without also tripping the "date just announced" milestone.
	st := &fakeStore{
		event: db.Event{ID: 1, TmEventID: "x", Name: "Test", PublicOnsaleAt: store.TS(public)},
		watch: activeWatch(),
	}

	fetch := &fakeFetcher{avail: "offsale", presale: &presale, publicStart: &public, publicEnd: &end, presaleName: "Artist"}
	notif := &fakeNotifier{}
	deps := Deps{Store: st, TM: fetch, Notifier: notif, Now: func() time.Time { return base }}

	// Presale is open, public sale is not yet.
	if err := ProcessEvent(context.Background(), 1, deps); err != nil {
		t.Fatal(err)
	}
	if got := st.kinds(1); len(got) != 1 || got[0] != "presale_open" {
		t.Fatalf("claimed kinds = %v, want [presale_open]", got)
	}
	if notif.calls != 1 {
		t.Fatalf("notifier calls = %d, want 1", notif.calls)
	}

	// Polling again while nothing changed must not re-alert.
	if err := ProcessEvent(context.Background(), 1, deps); err != nil {
		t.Fatal(err)
	}
	if notif.calls != 1 {
		t.Errorf("notifier calls = %d after a no-change poll, want still 1", notif.calls)
	}

	// Now the public sale opens: a second, different alert.
	later := public.Add(time.Minute)
	deps.Now = func() time.Time { return later }
	fetch.avail = "onsale"
	if err := ProcessEvent(context.Background(), 1, deps); err != nil {
		t.Fatal(err)
	}
	if got := st.kinds(1); len(got) != 2 {
		t.Fatalf("claimed kinds = %v, want presale_open and public_open", got)
	}
	if notif.calls != 2 {
		t.Errorf("notifier calls = %d, want 2 (presale then public)", notif.calls)
	}
	if notif.last.Kind != "public_open" {
		t.Errorf("last alert kind = %q, want public_open", notif.last.Kind)
	}
}

// The announcement fires when an unknown onsale date becomes a real one — and
// only once, even though the date stays known afterwards.
func TestProcessEvent_OnsaleDateAnnouncedFiresOnce(t *testing.T) {
	base := time.Now()
	st := &fakeStore{
		event: db.Event{ID: 1, TmEventID: "x", Name: "Test"}, // no stored onsale date
		watch: activeWatch(),
	}
	public := base.Add(7 * 24 * time.Hour)
	end := base.Add(60 * 24 * time.Hour)
	fetch := &fakeFetcher{avail: "offsale", publicStart: &public, publicEnd: &end}
	notif := &fakeNotifier{}
	deps := Deps{Store: st, TM: fetch, Notifier: notif, Now: func() time.Time { return base }}

	if err := ProcessEvent(context.Background(), 1, deps); err != nil {
		t.Fatal(err)
	}
	if notif.calls != 1 || notif.last.Kind != "onsale_announced" {
		t.Fatalf("calls=%d lastKind=%q, want 1 onsale_announced", notif.calls, notif.last.Kind)
	}
	// The date is stored now, so a second poll is not an announcement.
	if err := ProcessEvent(context.Background(), 1, deps); err != nil {
		t.Fatal(err)
	}
	if notif.calls != 1 {
		t.Errorf("notifier calls = %d, want 1 (announcement must not repeat)", notif.calls)
	}
}

// A cancelled event alerts once and then stops being polled, so the API budget
// isn't spent on something that can never change.
func TestProcessEvent_CancelledAlertsAndClosesWatches(t *testing.T) {
	base := time.Now()
	st := &fakeStore{
		event: db.Event{ID: 1, TmEventID: "x", Name: "Test"},
		watch: activeWatch(),
	}
	public := base.Add(-24 * time.Hour) // sale window was open
	end := base.Add(24 * time.Hour)
	fetch := &fakeFetcher{avail: "cancelled", publicStart: &public, publicEnd: &end}
	notif := &fakeNotifier{}
	deps := Deps{Store: st, TM: fetch, Notifier: notif, Now: func() time.Time { return base }}

	if err := ProcessEvent(context.Background(), 1, deps); err != nil {
		t.Fatal(err)
	}
	if got := st.kinds(1); len(got) != 1 || got[0] != "cancelled" {
		t.Errorf("claimed kinds = %v, want [cancelled] only — an open window must not also alert as on sale", got)
	}
	if st.closed == 0 {
		t.Error("CloseWatchesForEvent was not called for a cancelled event")
	}
}

// Past events must be closed without alerting at all.
func TestProcessEvent_PastEventClosesSilently(t *testing.T) {
	base := time.Now()
	past := base.Add(-48 * time.Hour)
	st := &fakeStore{
		event: db.Event{ID: 1, TmEventID: "x", Name: "Test", EventDate: store.TS(past)},
		watch: activeWatch(),
	}
	fetch := &fakeFetcher{avail: "onsale"}
	notif := &fakeNotifier{}
	deps := Deps{Store: st, TM: fetch, Notifier: notif, Now: func() time.Time { return base }}

	if err := ProcessEvent(context.Background(), 1, deps); err != nil {
		t.Fatal(err)
	}
	if notif.calls != 0 {
		t.Errorf("notifier calls = %d, want 0 for an event that already happened", notif.calls)
	}
	if st.closed == 0 {
		t.Error("watches for a past event were not closed — polling would continue forever")
	}
}
