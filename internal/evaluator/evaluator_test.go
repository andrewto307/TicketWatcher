package evaluator

import (
	"testing"
	"time"
)

var now = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

func ago(d time.Duration) *time.Time  { t := now.Add(-d); return &t }
func from(d time.Duration) *time.Time { t := now.Add(d); return &t }

func has(kinds []Kind, want Kind) bool {
	for _, k := range kinds {
		if k == want {
			return true
		}
	}
	return false
}

func TestEvaluate(t *testing.T) {
	day := 24 * time.Hour

	tests := []struct {
		name       string
		obs        Observation
		prior      Prior
		wantKinds  []Kind
		wantAbsent []Kind
		wantClose  bool
	}{
		{
			name: "nothing open yet — no alert, just waiting",
			obs: Observation{Now: now, Availability: "offsale",
				PublicStart: from(5 * day), EarliestPresale: from(3 * day), EventDate: from(90 * day)},
			prior:      Prior{KnewPublicStart: true},
			wantKinds:  nil,
			wantAbsent: []Kind{KindPresaleOpen, KindPublicOpen, KindSaleClosed},
		},
		{
			name: "presale just opened, public sale still ahead",
			obs: Observation{Now: now, Availability: "offsale",
				PublicStart: from(2 * day), EarliestPresale: ago(time.Hour), EventDate: from(90 * day)},
			prior:      Prior{KnewPublicStart: true},
			wantKinds:  []Kind{KindPresaleOpen},
			wantAbsent: []Kind{KindPublicOpen},
		},
		{
			name: "public sale open and status agrees",
			obs: Observation{Now: now, Availability: "onsale",
				PublicStart: ago(time.Hour), PublicEnd: from(30 * day),
				EarliestPresale: ago(3 * day), EventDate: from(60 * day)},
			prior:     Prior{KnewPublicStart: true},
			wantKinds: []Kind{KindPresaleOpen, KindPublicOpen},
		},
		{
			// The 6 measured "window open but sale ended" cases must not alert as open.
			name: "public window says open but status disagrees -> no public_open",
			obs: Observation{Now: now, Availability: "offsale",
				PublicStart: ago(10 * day), PublicEnd: from(day), EventDate: from(60 * day)},
			prior:      Prior{KnewPublicStart: true},
			wantAbsent: []Kind{KindPublicOpen, KindStatusOnsale},
		},
		{
			name: "sale closed -> one notice, and never alongside an open state",
			obs: Observation{Now: now, Availability: "offsale",
				PublicStart: ago(30 * day), PublicEnd: ago(day),
				EarliestPresale: ago(40 * day), EventDate: from(10 * day)},
			prior:      Prior{KnewPublicStart: true},
			wantKinds:  []Kind{KindSaleClosed},
			wantAbsent: []Kind{KindPresaleOpen, KindPublicOpen},
		},
		{
			name: "onsale date announced (sentinel replaced by a real date)",
			obs: Observation{Now: now, Availability: "offsale",
				PublicStart: from(7 * day), EventDate: from(90 * day)},
			prior:     Prior{KnewPublicStart: false},
			wantKinds: []Kind{KindOnsaleAnnounced},
		},
		{
			name: "already knew the date -> not an announcement",
			obs: Observation{Now: now, Availability: "offsale",
				PublicStart: from(7 * day), EventDate: from(90 * day)},
			prior:      Prior{KnewPublicStart: true},
			wantAbsent: []Kind{KindOnsaleAnnounced},
		},
		{
			// Cancelled beats every date trigger — the measured failure mode.
			name: "cancelled with an open sale window -> cancelled only",
			obs: Observation{Now: now, Availability: "cancelled",
				PublicStart: ago(10 * day), PublicEnd: from(30 * day),
				EarliestPresale: ago(20 * day), EventDate: from(30 * day)},
			prior:      Prior{KnewPublicStart: true},
			wantKinds:  []Kind{KindCancelled},
			wantAbsent: []Kind{KindPublicOpen, KindPresaleOpen, KindOnsaleAnnounced},
			wantClose:  true,
		},
		{
			name: "rescheduled with an open window -> rescheduled only, watch stays open",
			obs: Observation{Now: now, Availability: "rescheduled",
				PublicStart: ago(10 * day), PublicEnd: from(30 * day), EventDate: from(30 * day)},
			prior:      Prior{KnewPublicStart: true},
			wantKinds:  []Kind{KindRescheduled},
			wantAbsent: []Kind{KindPublicOpen},
			wantClose:  false,
		},
		{
			name: "postponed is treated like rescheduled",
			obs: Observation{Now: now, Availability: "postponed",
				PublicStart: ago(day), EventDate: from(30 * day)},
			prior:     Prior{KnewPublicStart: true},
			wantKinds: []Kind{KindRescheduled},
		},
		{
			// The ~4% of events with no announced onsale date.
			name: "unknown onsale date -> status flip is the fallback",
			obs: Observation{Now: now, Availability: "onsale",
				OnsaleTBD: true, PublicStart: nil, EventDate: from(60 * day)},
			prior:     Prior{LastEvaluation: false},
			wantKinds: []Kind{KindStatusOnsale},
		},
		{
			name: "unknown onsale date, already onsale last poll -> no re-alert",
			obs: Observation{Now: now, Availability: "onsale",
				OnsaleTBD: true, PublicStart: nil, EventDate: from(60 * day)},
			prior:      Prior{LastEvaluation: true},
			wantAbsent: []Kind{KindStatusOnsale},
		},
		{
			// Avoids double-alerting when both paths could apply.
			name: "with a usable public date, the status fallback stays quiet",
			obs: Observation{Now: now, Availability: "onsale",
				PublicStart: ago(time.Hour), PublicEnd: from(30 * day), EventDate: from(60 * day)},
			prior:      Prior{KnewPublicStart: true, LastEvaluation: false},
			wantKinds:  []Kind{KindPublicOpen},
			wantAbsent: []Kind{KindStatusOnsale},
		},
		{
			name: "event already happened -> close the watch, alert nothing",
			obs: Observation{Now: now, Availability: "onsale",
				PublicStart: ago(60 * day), PublicEnd: ago(day), EventDate: ago(day)},
			prior:      Prior{KnewPublicStart: true},
			wantKinds:  nil,
			wantAbsent: []Kind{KindSaleClosed, KindPublicOpen, KindStatusOnsale},
			wantClose:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.obs, tt.prior)

			for _, want := range tt.wantKinds {
				if !has(got.Kinds, want) {
					t.Errorf("missing kind %q; got %v", want, got.Kinds)
				}
			}
			for _, bad := range tt.wantAbsent {
				if has(got.Kinds, bad) {
					t.Errorf("unexpected kind %q; got %v", bad, got.Kinds)
				}
			}
			if tt.wantKinds == nil && len(tt.wantAbsent) == 0 && len(got.Kinds) != 0 {
				t.Errorf("expected no kinds, got %v", got.Kinds)
			}
			if got.CloseWatch != tt.wantClose {
				t.Errorf("CloseWatch = %v, want %v", got.CloseWatch, tt.wantClose)
			}
		})
	}
}

// StatusOnsale is persisted as last_evaluation, so it must reflect the raw status
// regardless of which branch returned.
func TestEvaluate_StatusOnsaleAlwaysReported(t *testing.T) {
	day := 24 * time.Hour
	cases := []struct {
		avail string
		want  bool
	}{{"onsale", true}, {"offsale", false}, {"cancelled", false}, {"rescheduled", false}}
	for _, c := range cases {
		got := Evaluate(Observation{Now: now, Availability: c.avail, EventDate: from(30 * day)}, Prior{})
		if got.StatusOnsale != c.want {
			t.Errorf("availability=%s StatusOnsale=%v, want %v", c.avail, got.StatusOnsale, c.want)
		}
	}
}

// Regression: a user unchecks the search filter and watches an event that is
// ALREADY on sale. Nothing has changed since they clicked Watch, so they must
// not be emailed at all — they saw that state on the search page.
func TestEvaluate_WatchingAnAlreadyOnsaleEventIsSilent(t *testing.T) {
	created := now.Add(-time.Minute) // they just added it
	res := Evaluate(Observation{
		Now:             now,
		Availability:    "onsale",
		PublicStart:     ago(72 * time.Hour),  // on sale for 3 days already
		PublicEnd:       from(30 * 24 * time.Hour),
		EarliestPresale: ago(96 * time.Hour),  // presale opened 4 days ago
		EventDate:       from(60 * 24 * time.Hour),
	}, Prior{
		KnewPublicStart: false, // brand-new event, nothing stored
		LastEvaluation:  true,  // seeded at creation because it was already onsale
		WatchCreatedAt:  created,
	})

	if len(res.Kinds) != 0 {
		t.Errorf("expected no alerts for an already-onsale event, got %v", res.Kinds)
	}
}

// An onsale date in the past is not an announcement.
func TestEvaluate_PastOnsaleDateIsNotAnAnnouncement(t *testing.T) {
	res := Evaluate(Observation{
		Now: now, Availability: "offsale",
		PublicStart: ago(72 * time.Hour), PublicEnd: from(24 * time.Hour),
		EventDate: from(30 * 24 * time.Hour),
	}, Prior{KnewPublicStart: false})

	if has(res.Kinds, KindOnsaleAnnounced) {
		t.Errorf("announced a date that already passed: %v", res.Kinds)
	}
}

// But a future date we didn't know about IS an announcement, even on a fresh watch.
func TestEvaluate_FutureOnsaleDateStillAnnounces(t *testing.T) {
	res := Evaluate(Observation{
		Now: now, Availability: "offsale",
		PublicStart: from(7 * 24 * time.Hour),
		EventDate:   from(90 * 24 * time.Hour),
	}, Prior{KnewPublicStart: false, WatchCreatedAt: now.Add(-time.Minute)})

	if !has(res.Kinds, KindOnsaleAnnounced) {
		t.Errorf("a future onsale date should still announce, got %v", res.Kinds)
	}
}

// A presale that opens AFTER the user subscribes must still alert — the
// suppression is about history, not about muting the watch.
func TestEvaluate_PresaleOpeningAfterSubscribingStillAlerts(t *testing.T) {
	created := now.Add(-48 * time.Hour)
	res := Evaluate(Observation{
		Now: now, Availability: "offsale",
		PublicStart:     from(5 * 24 * time.Hour),
		EarliestPresale: ago(time.Hour), // opened an hour ago, well after they subscribed
		EventDate:       from(60 * 24 * time.Hour),
	}, Prior{KnewPublicStart: true, WatchCreatedAt: created})

	if !has(res.Kinds, KindPresaleOpen) {
		t.Errorf("presale opening after subscription must alert, got %v", res.Kinds)
	}
}
