// Package evaluator decides whether a watch's alert condition currently holds.
// It is a pure function of (condition, observation) with no I/O, so it is trivially
// unit-testable and has no dependencies on the database or the API client.
package evaluator

// Condition is a watch's alert rule.
type Condition struct {
	Type           string // "price_below" | "becomes_available"
	ThresholdCents *int64 // required for price_below
}

// Observation is the latest known state of an event (money in integer cents).
type Observation struct {
	MinPriceCents *int64
	Availability  string
}

// Met reports whether the condition currently holds for the observation.
func Met(c Condition, o Observation) bool {
	switch c.Type {
	case "price_below":
		// Not met if we have no threshold, or no observed price to compare.
		if c.ThresholdCents == nil || o.MinPriceCents == nil {
			return false
		}
		return *o.MinPriceCents < *c.ThresholdCents
	case "becomes_available":
		return o.Availability == "onsale"
	default:
		return false
	}
}
