//go:build integration

package inttest

import (
	"context"
	"strings"
	"testing"
	"time"

	"ticket-watcher/internal/store/db"
)

// Edge cases that nobody has manually exercised yet. Written to find bugs, not to
// confirm the happy path — several of these encode behaviour that was previously
// undefined, so a failure here is a genuine question about intent, not noise.

// Two users watching the same event must each be told once. Dedupe is keyed by
// (watch, kind), so it must not let one user's alert suppress another's.
func TestEdge_TwoUsersOnOneEventBothGetAlerted(t *testing.T) {
	h := newHarness(t)
	alice := h.user("alice")
	bob := h.user("bob")

	publicAt := h.now.Add(2 * time.Hour)
	ev := eventSpec{
		ID: "TM-SHARED", Name: "Shared Show", Status: "offsale",
		EventDate: h.inFuture(60 * day), PublicStart: &publicAt, PublicEnd: h.inFuture(50 * day),
	}
	wa := h.watch(alice, ev)
	wb := h.watch(bob, ev)

	h.advance(3 * time.Hour)
	ev.Status = "onsale"
	h.poll(ev)

	h.wantEmailCount(2, "one alert per watching user")
	got := map[string]bool{}
	for _, e := range h.emails() {
		got[e.To] = true
	}
	if !got[alice.Email] || !got[bob.Email] {
		t.Errorf("both users must be emailed; got recipients %v", got)
	}
	if k := h.alertKinds(wa.ID); !hasKind(k, "public_open") {
		t.Errorf("alice's watch missing public_open: %v", k)
	}
	if k := h.alertKinds(wb.ID); !hasKind(k, "public_open") {
		t.Errorf("bob's watch missing public_open: %v", k)
	}

	h.poll(ev)
	h.wantEmailCount(2, "neither user should be alerted twice")
}

// A user who subscribes AFTER a milestone has passed must not receive it, while a
// user who subscribed before must. This is the per-watch suppression working
// against a shared event row.
func TestEdge_LateSubscriberDoesNotGetHistoricMilestones(t *testing.T) {
	h := newHarness(t)
	early := h.user("early")

	publicAt := h.now.Add(time.Hour)
	ev := eventSpec{
		ID: "TM-LATE", Name: "Late Joiner", Status: "offsale",
		EventDate: h.inFuture(60 * day), PublicStart: &publicAt, PublicEnd: h.inFuture(50 * day),
	}
	h.watch(early, ev)

	// The sale opens; the early subscriber is told.
	h.advance(2 * time.Hour)
	ev.Status = "onsale"
	h.poll(ev)
	h.wantEmailCount(1, "early subscriber alerted")

	// A second user subscribes now — the sale is already open, so it isn't news.
	late := h.user("late")
	wl := h.watch(late, ev)
	h.poll(ev)

	h.wantEmailCount(1, "late subscriber must not be told about a sale that opened before they joined")
	if k := h.alertKinds(wl.ID); hasKind(k, "public_open") {
		t.Errorf("late subscriber claimed public_open for a pre-existing state: %v", k)
	}
}

// Pausing and resuming a watch re-arms it (reset_evaluation). This documents what
// actually happens, because a naive re-arm could replay milestones the user has
// already been emailed about.
func TestEdge_PauseResumeDoesNotReplayAlerts(t *testing.T) {
	h := newHarness(t)
	u := h.user("pauser")
	ctx := context.Background()

	publicAt := h.now.Add(time.Hour)
	ev := eventSpec{
		ID: "TM-PAUSE", Name: "Pause Me", Status: "offsale",
		EventDate: h.inFuture(60 * day), PublicStart: &publicAt, PublicEnd: h.inFuture(50 * day),
	}
	v := h.watch(u, ev)

	h.advance(2 * time.Hour)
	ev.Status = "onsale"
	h.poll(ev)
	h.wantEmailCount(1, "sale opened")

	// Pause, then resume — which resets last_evaluation.
	paused, active := "paused", "active"
	if _, err := h.svc.Update(ctx, u.ID, v.ID, serviceUpdate(paused)); err != nil {
		t.Fatalf("pause: %v", err)
	}
	h.poll(ev) // paused watches are not evaluated at all
	h.wantEmailCount(1, "a paused watch must not alert")

	if _, err := h.svc.Update(ctx, u.ID, v.ID, serviceUpdate(active)); err != nil {
		t.Fatalf("resume: %v", err)
	}
	h.poll(ev)
	h.wantEmailCount(1, "resuming must not replay an alert the user already received")
}

// The same, for an event whose onsale date is unknown — the status-flip fallback
// path, which is NOT protected by the watch_alerts table and relies purely on
// last_evaluation.
func TestEdge_PauseResumeOnUnknownDateEventDoesNotReplay(t *testing.T) {
	h := newHarness(t)
	u := h.user("pauser-tbd")
	ctx := context.Background()

	ev := eventSpec{
		ID: "TM-PAUSE-TBD", Name: "Pause Me, Date Unknown", Status: "offsale",
		EventDate: h.inFuture(60 * day), SentinelOnsale: true,
	}
	v := h.watch(u, ev)
	h.poll(ev)
	h.wantNoEmails("not on sale yet")

	// It goes on sale; the fallback fires.
	ev.Status = "onsale"
	h.poll(ev)
	h.wantEmailCount(1, "status flipped to onsale")

	paused, active := "paused", "active"
	if _, err := h.svc.Update(ctx, u.ID, v.ID, serviceUpdate(paused)); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if _, err := h.svc.Update(ctx, u.ID, v.ID, serviceUpdate(active)); err != nil {
		t.Fatalf("resume: %v", err)
	}
	h.poll(ev)

	h.wantEmailCount(1, "resuming an unknown-date watch must not replay 'on sale now'")
}

// A watch whose owner hasn't verified their email: the alert is withheld. This
// asserts the *consequence* — the milestone is consumed, so verifying later does
// not retroactively deliver it.
func TestEdge_UnverifiedUserLosesTheAlertPermanently(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	u, err := h.q.CreateUser(ctx, db.CreateUserParams{
		Email: uniqueEmail("unverified"), PasswordHash: "x",
	})
	if err != nil {
		t.Fatal(err)
	} // deliberately NOT verified

	publicAt := h.now.Add(time.Hour)
	ev := eventSpec{
		ID: "TM-UNVERIFIED", Name: "Unverified Owner", Status: "offsale",
		EventDate: h.inFuture(60 * day), PublicStart: &publicAt, PublicEnd: h.inFuture(50 * day),
	}
	v := h.watch(u, ev)

	h.advance(2 * time.Hour)
	ev.Status = "onsale"
	h.poll(ev)
	h.wantNoEmails("owner has not verified their address")

	// The milestone is recorded as handled even though nothing was sent.
	if k := h.alertKinds(v.ID); !hasKind(k, "public_open") {
		t.Errorf("expected the milestone to be claimed even when withheld, got %v", k)
	}

	// Verifying afterwards does not resend it — a deliberate trade-off, recorded
	// here so the behaviour is a decision rather than a surprise.
	if err := h.q.MarkEmailVerified(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	h.poll(ev)
	h.wantNoEmails("verifying later does not replay a withheld milestone")
}

// An unsubscribed user receives nothing, but the watch keeps tracking so that
// resubscribing works.
func TestEdge_UnsubscribedUserGetsNothingButWatchKeepsTracking(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	u := h.user("optout")
	if err := h.q.SetUnsubscribed(ctx, u.ID); err != nil {
		t.Fatal(err)
	}

	publicAt := h.now.Add(time.Hour)
	ev := eventSpec{
		ID: "TM-OPTOUT", Name: "Opted Out", Status: "offsale",
		EventDate: h.inFuture(60 * day), PublicStart: &publicAt, PublicEnd: h.inFuture(50 * day),
	}
	v := h.watch(u, ev)

	h.advance(2 * time.Hour)
	ev.Status = "onsale"
	h.poll(ev)

	h.wantNoEmails("user has unsubscribed")
	if w := h.reloadWatch(v.ID, u.ID); w.Status != "active" {
		t.Errorf("watch status = %q, want active — unsubscribing mutes email, it doesn't stop tracking", w.Status)
	}
	// The event's observed state still advanced, so the dashboard stays correct.
	if e := h.reloadEvent(ev.ID); e.LastAvailability == nil || *e.LastAvailability != "onsale" {
		t.Errorf("event state not updated for an unsubscribed watcher: %v", e.LastAvailability)
	}
}

// Ticketmaster sometimes moves an announced onsale date. This records what the
// app currently does so the behaviour is deliberate rather than accidental.
func TestEdge_OnsaleDateMovedIsStoredEvenIfNotAlerted(t *testing.T) {
	h := newHarness(t)
	u := h.user("moved")

	first := h.now.Add(5 * day)
	ev := eventSpec{
		ID: "TM-MOVED", Name: "Moving Target", Status: "offsale",
		EventDate: h.inFuture(60 * day), PublicStart: &first, PublicEnd: h.inFuture(50 * day),
	}
	h.watch(u, ev)
	h.poll(ev)
	h.wantNoEmails("date known, sale not open")

	// The promoter pushes the onsale back a week.
	moved := h.now.Add(12 * day)
	ev.PublicStart = &moved
	h.poll(ev)

	// No alert today — but the new date MUST be stored, or the dashboard would
	// show a stale time and the sale would open without warning.
	h.wantNoEmails("a moved onsale date is not currently alerted")
	e := h.reloadEvent(ev.ID)
	if !e.PublicOnsaleAt.Valid || !e.PublicOnsaleAt.Time.UTC().Truncate(time.Second).Equal(moved.UTC().Truncate(time.Second)) {
		t.Errorf("stored onsale date = %v, want the updated %v", e.PublicOnsaleAt.Time, moved)
	}

	// And when the new date arrives, the alert still fires.
	h.advance(13 * day)
	ev.Status = "onsale"
	h.poll(ev)
	h.wantEmailCount(1, "sale opened at the revised time")
}

// Polling an event with no watches at all must not panic or send anything.
func TestEdge_EventWithOnlyPausedWatchesIsHarmless(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	u := h.user("allpaused")

	ev := eventSpec{
		ID: "TM-NOWATCH", Name: "Nobody Watching", Status: "offsale",
		EventDate: h.inFuture(30 * day), PublicStart: h.inFuture(day), PublicEnd: h.inFuture(29 * day),
	}
	v := h.watch(u, ev)
	paused := "paused"
	if _, err := h.svc.Update(ctx, u.ID, v.ID, serviceUpdate(paused)); err != nil {
		t.Fatal(err)
	}

	h.advance(2 * day)
	ev.Status = "onsale"
	h.poll(ev) // must not panic
	h.wantNoEmails("all watches paused")
}

// An event name containing HTML must survive into the email safely. Names come
// straight from the Ticketmaster API.
func TestEdge_HostileEventNameIsEscapedInEmail(t *testing.T) {
	h := newHarness(t)
	u := h.user("hostile")

	publicAt := h.now.Add(time.Hour)
	ev := eventSpec{
		ID: "TM-XSS", Name: `Rock & Roll <script>alert(1)</script>`, Status: "offsale",
		EventDate: h.inFuture(60 * day), PublicStart: &publicAt, PublicEnd: h.inFuture(50 * day),
	}
	h.watch(u, ev)

	h.advance(2 * time.Hour)
	ev.Status = "onsale"
	h.poll(ev)

	h.wantEmailCount(1, "sale opened")
	e := h.emails()[0]
	if strings.Contains(e.HTML, "<script>") {
		t.Errorf("unescaped markup from the event name reached the HTML email:\n%s", e.HTML)
	}
	if !strings.Contains(e.HTML, "&amp;") {
		t.Errorf("ampersand not escaped in HTML body:\n%s", e.HTML)
	}
	if !strings.Contains(e.Text, "Rock & Roll") {
		t.Errorf("plain-text body should keep the original characters:\n%s", e.Text)
	}
}

// The watch cap must be enforced through the real service, not just the handler.
func TestEdge_WatchCapEnforcedInService(t *testing.T) {
	h := newHarness(t)
	u := h.user("capped")
	h.svc = newCappedWatchService(h, 1)

	ev1 := eventSpec{ID: "TM-CAP-1", Name: "One", Status: "offsale", EventDate: h.inFuture(30 * day)}
	ev2 := eventSpec{ID: "TM-CAP-2", Name: "Two", Status: "offsale", EventDate: h.inFuture(30 * day)}

	h.watch(u, ev1)

	h.stub.set(ev2.json())
	_, err := h.svc.Create(context.Background(), u.ID, createInput(ev2.ID))
	if err == nil {
		t.Fatal("second watch was allowed despite a cap of 1")
	}
	if !strings.Contains(err.Error(), "up to 1") {
		t.Errorf("error should state the limit, got %q", err)
	}

	// Crucially, a refused watch must not have cost a Ticketmaster call for an
	// event we were never going to track.
	before := h.stub.callCount()
	_, _ = h.svc.Create(context.Background(), u.ID, createInput("TM-CAP-3"))
	if got := h.stub.callCount(); got != before {
		t.Errorf("an over-cap request made %d API call(s); the cap must be checked first", got-before)
	}
}
