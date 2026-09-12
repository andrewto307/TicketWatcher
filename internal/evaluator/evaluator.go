// Package evaluator decides whether a watch's alert condition currently holds.
// It is a pure function of (condition, observation) with no I/O, so it is trivially
// unit-testable and has no dependencies on the database or the API client.
package evaluator

// Condition is a watch's alert rule.
//
// Only "becomes_available" exists. A "price_below" condition was removed when
// Ticketmaster dropped priceRanges from the Discovery API (2025-03-11) — see
// plan/06-design-decisions.md D13. The Type field is kept rather than collapsed
// away because this purity is what made removing a condition a one-case change,
// and it leaves the same room for the next one.
type Condition struct {
	Type string // "becomes_available"
}

// Observation is the latest known state of an event.
type Observation struct {
	Availability string // onsale | offsale | cancelled | postponed | rescheduled | unknown
}

// Met reports whether the condition currently holds for the observation.
//
// Note what "onsale" means: the sale window is open, not that seats are in stock —
// a sold-out show still reports onsale. Real inventory lives in Ticketmaster's
// partner-only Inventory Status API. So this detects the offsale -> onsale
// transition, which is what the product promises.
func Met(c Condition, o Observation) bool {
	switch c.Type {
	case "becomes_available":
		return o.Availability == "onsale"
	default:
		return false
	}
}
