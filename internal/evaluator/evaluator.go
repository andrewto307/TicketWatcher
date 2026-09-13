// Package evaluator decides which sale milestones an event has reached and which
// of them are worth telling a user about.
//
// It is a pure function of (observation, prior state) with no I/O, so every branch
// is testable without a database or an API. That purity is also what made removing
// price watching (D13) a one-case change, and what would make adding Ticketmaster's
// partner-only resaleStatus another one.
//
// The rules and the live measurements behind them are documented in
// plan/08-sale-milestone-alerts.md.
package evaluator

import "time"

// Kind identifies one alertable milestone. Values are persisted in
// watch_alerts.kind, so they are part of the data contract — don't rename them.
type Kind string

const (
	// KindCancelled and KindRescheduled are vetoes: they suppress sale alerts and
	// are themselves worth one notice, because a user waiting on the event needs
	// to stop waiting.
	KindCancelled   Kind = "cancelled"
	KindRescheduled Kind = "rescheduled"

	// KindOnsaleAnnounced fires when Ticketmaster replaces its "date not
	// announced" sentinel with a real onsale date. 4% of events sit in that
	// state, including high-demand shows, so this is often the first useful thing
	// we can tell anyone.
	KindOnsaleAnnounced Kind = "onsale_announced"

	// KindPresaleOpen is the earliest moment a user can actually buy.
	KindPresaleOpen Kind = "presale_open"

	// KindPublicOpen is the general-public sale opening, confirmed by status.
	KindPublicOpen Kind = "public_open"

	// KindStatusOnsale is the fallback for events whose onsale date is unknown:
	// with no dates to compare, an offsale -> onsale status flip is all we have.
	KindStatusOnsale Kind = "status_onsale"

	// KindSaleClosed tells the user to stop waiting on the official sale and
	// points them at resale, which we cannot observe.
	KindSaleClosed Kind = "sale_closed"
)

// Observation is what the latest poll saw.
type Observation struct {
	Now          time.Time
	Availability string // onsale | offsale | cancelled | postponed | rescheduled | unknown

	PublicStart *time.Time // nil when unknown or TBD
	PublicEnd   *time.Time
	OnsaleTBD   bool // the API said "date not announced"

	EarliestPresale *time.Time

	EventDate *time.Time // when the event itself happens
}

// Prior is the state carried from previous polls, so a transition can be
// distinguished from a steady state.
type Prior struct {
	// KnewPublicStart is true if a real public onsale date was already stored.
	// Going from false -> true is what makes an announcement an announcement.
	KnewPublicStart bool

	// LastEvaluation is the previous result of the status-flip fallback, i.e. was
	// Availability == "onsale" last time. Mirrors watches.last_evaluation.
	LastEvaluation bool
}

// Result is the verdict for one poll.
type Result struct {
	// Kinds are the milestones that hold right now, in the order they should be
	// reported. The caller is responsible for firing each at most once per watch.
	Kinds []Kind

	// StatusOnsale is the current value of the fallback signal; the caller
	// persists it as last_evaluation for the next poll's edge detection.
	StatusOnsale bool

	// CloseWatch is true once the event is in the past: there is nothing left to
	// alert on, so polling it only spends API budget.
	CloseWatch bool
}

// Evaluate applies the veto-then-trigger rules from plan/08.
func Evaluate(o Observation, p Prior) Result {
	res := Result{StatusOnsale: o.Availability == "onsale"}

	// --- veto: the event already happened ---
	// Checked first and returns nothing: no milestone matters for a past event,
	// and continuing to poll it is pure waste.
	if o.EventDate != nil && o.EventDate.Before(o.Now) {
		res.CloseWatch = true
		return res
	}

	// --- veto: cancelled / moved ---
	// These must beat every date-based trigger. Measured on live data: 8 of 1,000
	// events were cancelled or rescheduled while their public sale window was
	// open, so date logic alone would have announced "on sale now" for a
	// cancelled show.
	switch o.Availability {
	case "cancelled":
		res.Kinds = []Kind{KindCancelled}
		res.CloseWatch = true // nothing further can happen
		return res
	case "postponed", "rescheduled":
		// Not closed: the event usually returns with a new date, and the user
		// still wants to hear about that.
		res.Kinds = []Kind{KindRescheduled}
		return res
	}

	// --- trigger: the onsale date just got announced ---
	if !p.KnewPublicStart && o.PublicStart != nil {
		res.Kinds = append(res.Kinds, KindOnsaleAnnounced)
	}

	// --- veto: the official sale has closed ---
	// Returns here so we never say "closed" and "open" in the same breath.
	if o.PublicEnd != nil && o.PublicEnd.Before(o.Now) {
		res.Kinds = append(res.Kinds, KindSaleClosed)
		return res
	}

	// --- trigger: a presale is open now ---
	// Only the earliest window is considered (some events have a dozen).
	if o.EarliestPresale != nil && !o.EarliestPresale.After(o.Now) {
		res.Kinds = append(res.Kinds, KindPresaleOpen)
	}

	// --- trigger: the public sale is open ---
	// Requires status agreement. On live data status and dates agreed in 880/880
	// onsale cases, so demanding both costs nothing and blocks the disagreeing
	// cases (sale ended, event withdrawn) from becoming false alerts.
	if o.PublicStart != nil && !o.PublicStart.After(o.Now) && res.StatusOnsale {
		res.Kinds = append(res.Kinds, KindPublicOpen)
	}

	// --- fallback: no usable dates, so trust the status flip ---
	// Applies to the ~4% of events whose onsale date is unannounced. Edge-
	// triggered on false -> true so a steady "onsale" doesn't re-alert.
	if o.PublicStart == nil && res.StatusOnsale && !p.LastEvaluation {
		res.Kinds = append(res.Kinds, KindStatusOnsale)
	}

	return res
}
