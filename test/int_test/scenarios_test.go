//go:build integration

package inttest

import (
	"strings"
	"testing"
	"time"
)

const day = 24 * time.Hour

// ─────────────────────────────────────────────────────────────────────────────
// The headline journey: a user watches an event before anything has opened, and
// is walked through the whole sale lifecycle exactly once per milestone.
// ─────────────────────────────────────────────────────────────────────────────

func TestScenario_FullLifecycle_NothingOpen_Presale_PublicSale(t *testing.T) {
	h := newHarness(t)
	u := h.user("lifecycle@example.test")

	presaleAt := h.now.Add(2 * time.Hour)
	publicAt := h.now.Add(2 * day)
	publicEnd := h.now.Add(60 * day)
	eventAt := h.now.Add(90 * day)

	ev := eventSpec{
		ID: "TM-LIFE", Name: "Lifecycle Fest", Status: "offsale",
		EventDate: &eventAt, PublicStart: &publicAt, PublicEnd: &publicEnd,
		Presales: []presaleSpec{{Name: "Artist Presale", Start: presaleAt, End: publicAt}},
	}

	// --- step 1: subscribe while nothing is open ---
	v := h.watch(u, ev)
	if v.SaleState != "onsale_scheduled" {
		t.Errorf("sale_state on create = %q, want onsale_scheduled", v.SaleState)
	}
	// The snapshot must be stored at creation, not rediscovered later.
	if v.LastPolledAt == nil {
		t.Error(`watch created with last_polled_at nil — the snapshot was discarded again`)
	}
	if v.PublicOnsaleStart == nil {
		t.Error("watch created without the onsale date we just fetched")
	}

	h.poll(ev)
	h.wantNoEmails("nothing is open yet")

	// --- step 2: the presale opens ---
	h.advance(3 * time.Hour)
	h.poll(ev)
	h.wantEmailCount(1, "presale opened")
	h.wantLastSubjectContains("presale open", "presale opened")

	// Polling repeatedly while the presale stays open must not re-alert.
	h.poll(ev)
	h.poll(ev)
	h.wantEmailCount(1, "presale still open across further polls")

	// --- step 3: the public sale opens ---
	h.advance(2 * day)
	ev.Status = "onsale"
	h.poll(ev)
	h.wantEmailCount(2, "public sale opened")
	h.wantLastSubjectContains("on sale now", "public sale opened")

	h.poll(ev)
	h.wantEmailCount(2, "still on sale; must not repeat")

	// --- the database's own record of what the user was told ---
	kinds := h.alertKinds(v.ID)
	if len(kinds) != 2 || !hasKind(kinds, "presale_open") || !hasKind(kinds, "public_open") {
		t.Errorf("recorded alert kinds = %v, want exactly [presale_open public_open]", kinds)
	}

	// Every alert must carry a working opt-out.
	for _, e := range h.emails() {
		if !strings.Contains(e.Text, "/api/unsubscribe?token=") {
			t.Errorf("email %q has no unsubscribe link", e.Subject)
		}
		if e.Headers["List-Unsubscribe"] == "" {
			t.Errorf("email %q missing List-Unsubscribe header", e.Subject)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Regression: watching something already on sale must be silent. This is the bug
// found by hand — it fired three emails at once, including an "onsale date
// announced" for a date days in the past.
// ─────────────────────────────────────────────────────────────────────────────

func TestScenario_WatchingAnAlreadyOnsaleEventIsSilent(t *testing.T) {
	h := newHarness(t)
	u := h.user("already-onsale@example.test")

	ev := eventSpec{
		ID: "TM-LIVE", Name: "Already Selling", Status: "onsale",
		EventDate:   h.inFuture(60 * day),
		PublicStart: h.inPast(3 * day), // on sale for days
		PublicEnd:   h.inFuture(30 * day),
		Presales:    []presaleSpec{{Name: "Fan Presale", Start: h.now.Add(-5 * day), End: h.now.Add(-3 * day)}},
	}

	v := h.watch(u, ev)
	if v.SaleState != "on_sale" {
		t.Errorf("sale_state = %q, want on_sale (not 'checking')", v.SaleState)
	}

	h.poll(ev)
	h.poll(ev)
	h.wantNoEmails("watching an event that was already on sale")

	if kinds := h.alertKinds(v.ID); len(kinds) != 0 {
		t.Errorf("alert kinds = %v, want none — nothing changed since subscribing", kinds)
	}

	// But a genuine future change must still reach the user.
	h.advance(31 * day)
	ev.Status = "offsale"
	h.poll(ev) // sale window has now closed
	h.wantEmailCount(1, "sale closed after subscribing")
	h.wantLastSubjectContains("sale closed", "sale closed")
}

// The same silence must hold when the onsale date is unannounced, which is the
// path that slipped through the first fix: last_evaluation had to be seeded from
// stored availability, and that availability wasn't being stored.
func TestScenario_AlreadyOnsaleWithUnannouncedDateIsSilent(t *testing.T) {
	h := newHarness(t)
	u := h.user("onsale-tbd@example.test")

	ev := eventSpec{
		ID: "TM-TBD-LIVE", Name: "Selling, Date Unknown", Status: "onsale",
		EventDate: h.inFuture(60 * day), SentinelOnsale: true,
	}

	v := h.watch(u, ev)
	h.poll(ev)
	h.poll(ev)
	h.wantNoEmails("already on sale with an unannounced onsale date")

	w := h.reloadWatch(v.ID, u.ID)
	if !w.LastEvaluation {
		t.Error("last_evaluation = false for an event that was on sale at creation — " +
			"the status-flip fallback will fire spuriously on the next poll")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// The 9999 sentinel: never a real date, and the announcement fires when a real
// date appears.
// ─────────────────────────────────────────────────────────────────────────────

func TestScenario_SentinelThenDateAnnounced(t *testing.T) {
	h := newHarness(t)
	u := h.user("announce@example.test")

	ev := eventSpec{
		ID: "TM-ANN", Name: "Date To Be Announced", Status: "offsale",
		EventDate: h.inFuture(120 * day), SentinelOnsale: true,
	}

	v := h.watch(u, ev)
	if v.SaleState != "onsale_tbd" {
		t.Errorf("sale_state = %q, want onsale_tbd", v.SaleState)
	}
	if v.PublicOnsaleStart != nil {
		t.Errorf("public_onsale_start = %v, want nil — the 9999 sentinel must never "+
			"become a real date", v.PublicOnsaleStart)
	}
	if !v.OnsaleTBD {
		t.Error("onsale_tbd = false; 'not announced' must be distinguishable from 'no data'")
	}

	h.poll(ev)
	h.wantNoEmails("still unannounced")

	// Ticketmaster publishes a date.
	h.advance(2 * day)
	announced := h.now.Add(10 * day)
	ev.SentinelOnsale, ev.PublicStart = false, &announced
	ev.PublicEnd = h.inFuture(100 * day)

	h.poll(ev)
	h.wantEmailCount(1, "onsale date announced")
	h.wantLastSubjectContains("onsale date announced", "announcement")

	// The announced date must appear in the body — an announcement without the
	// date is useless.
	last := h.emails()[0]
	if !strings.Contains(last.Text, announced.UTC().Format("2 Jan 2006")) {
		t.Errorf("announcement doesn't state the date:\n%s", last.Text)
	}

	h.poll(ev)
	h.wantEmailCount(1, "date already known; must not re-announce")

	// And when that date arrives, the public-sale alert still fires.
	h.advance(11 * day)
	ev.Status = "onsale"
	h.poll(ev)
	h.wantEmailCount(2, "public sale opened after announcement")
	h.wantLastSubjectContains("on sale now", "public open after announcement")
}

// ─────────────────────────────────────────────────────────────────────────────
// Vetoes: cancelled and rescheduled must beat every date-based trigger. Measured
// on live data, 8 of 1,000 events were cancelled or rescheduled while their
// public sale window was open.
// ─────────────────────────────────────────────────────────────────────────────

func TestScenario_CancelledBeatsAnOpenSaleWindow(t *testing.T) {
	h := newHarness(t)
	u := h.user("cancelled@example.test")

	ev := eventSpec{
		ID: "TM-CANCEL", Name: "Doomed Show", Status: "offsale",
		EventDate: h.inFuture(30 * day), PublicStart: h.inFuture(day), PublicEnd: h.inFuture(29 * day),
	}
	v := h.watch(u, ev)
	h.poll(ev)
	h.wantNoEmails("not open yet")

	// The sale window opens AND the event is cancelled.
	h.advance(2 * day)
	ev.Status = "cancelled"
	h.poll(ev)

	h.wantEmailCount(1, "cancelled")
	h.wantLastSubjectContains("cancelled", "cancellation")
	if kinds := h.alertKinds(v.ID); hasKind(kinds, "public_open") {
		t.Errorf("alerted public_open for a CANCELLED event: %v", kinds)
	}

	// Watches are closed, so the scheduler stops spending budget on it.
	w := h.reloadWatch(v.ID, u.ID)
	if w.Status != "paused" {
		t.Errorf("watch status = %q after cancellation, want paused", w.Status)
	}
	h.poll(ev)
	h.wantEmailCount(1, "cancellation must not repeat")
}

func TestScenario_RescheduledAlertsButKeepsWatching(t *testing.T) {
	h := newHarness(t)
	u := h.user("resched@example.test")

	ev := eventSpec{
		ID: "TM-RESCHED", Name: "Moved Show", Status: "rescheduled",
		EventDate: h.inFuture(40 * day), PublicStart: h.inPast(day), PublicEnd: h.inFuture(39 * day),
		Presales: []presaleSpec{{Name: "Citi Presale", Start: h.now.Add(-2 * day), End: h.now.Add(-day)}},
	}
	v := h.watch(u, ev)
	h.poll(ev)

	h.wantEmailCount(1, "rescheduled")
	h.wantLastSubjectContains("date changed", "reschedule notice")

	kinds := h.alertKinds(v.ID)
	if hasKind(kinds, "public_open") || hasKind(kinds, "presale_open") {
		t.Errorf("a rescheduled event alerted sale milestones: %v", kinds)
	}

	// Unlike cancellation, the watch stays active — the event usually returns.
	if w := h.reloadWatch(v.ID, u.ID); w.Status != "active" {
		t.Errorf("watch status = %q after reschedule, want active", w.Status)
	}

	// When it comes back on sale, the user hears about it.
	h.advance(day)
	ev.Status = "onsale"
	newStart := h.now.Add(-time.Hour)
	ev.PublicStart = &newStart
	h.poll(ev)
	h.wantEmailCount(2, "back on sale after reschedule")
	h.wantLastSubjectContains("on sale now", "resumed sale")
}

// ─────────────────────────────────────────────────────────────────────────────
// A past event must stop being polled, silently. This protects the shared API
// budget from events that can never change again.
// ─────────────────────────────────────────────────────────────────────────────

func TestScenario_PastEventClosesWatchesSilently(t *testing.T) {
	h := newHarness(t)
	u := h.user("past@example.test")

	ev := eventSpec{
		ID: "TM-PAST", Name: "Soon Over", Status: "onsale",
		EventDate: h.inFuture(time.Hour), PublicStart: h.inPast(10 * day), PublicEnd: h.inFuture(time.Hour),
	}
	v := h.watch(u, ev)
	h.poll(ev)
	h.wantNoEmails("already on sale when watched")

	// The event happens.
	h.advance(2 * time.Hour)
	h.poll(ev)

	h.wantNoEmails("event is over — nothing to say")
	if w := h.reloadWatch(v.ID, u.ID); w.Status != "paused" {
		t.Errorf("watch status = %q after the event passed, want paused so polling stops", w.Status)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// The sale-closed message is the one place the app must point users at resale,
// because it cannot see resale inventory itself.
// ─────────────────────────────────────────────────────────────────────────────

func TestScenario_SaleClosedGuidesToResale(t *testing.T) {
	h := newHarness(t)
	u := h.user("closed@example.test")

	ev := eventSpec{
		ID: "TM-CLOSED", Name: "Sold Through", Status: "offsale",
		EventDate: h.inFuture(20 * day), PublicStart: h.inFuture(day), PublicEnd: h.inFuture(2 * day),
	}
	v := h.watch(u, ev)

	h.advance(3 * day) // past the end of the sale window
	h.poll(ev)

	h.wantEmailCount(1, "sale closed")
	e := h.emails()[0]
	if !strings.Contains(strings.ToLower(e.Text), "resale") {
		t.Errorf("sale-closed email must point at resale:\n%s", e.Text)
	}
	if !strings.Contains(e.Text, "ticketmaster.test/e/TM-CLOSED") {
		t.Errorf("sale-closed email must link to the event page:\n%s", e.Text)
	}
	if kinds := h.alertKinds(v.ID); !hasKind(kinds, "sale_closed") {
		t.Errorf("alert kinds = %v, want sale_closed", kinds)
	}

	h.poll(ev)
	h.wantEmailCount(1, "sale-closed must not repeat")
}
