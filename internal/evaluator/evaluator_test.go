package evaluator

import "testing"

func TestMet_BecomesAvailable(t *testing.T) {
	tests := []struct {
		name         string
		availability string
		want         bool
	}{
		{"on sale", "onsale", true},
		{"off sale", "offsale", false},
		{"cancelled", "cancelled", false},
		{"postponed", "postponed", false},
		{"rescheduled", "rescheduled", false},
		{"unknown", "unknown", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Met(
				Condition{Type: "becomes_available"},
				Observation{Availability: tt.availability},
			)
			if got != tt.want {
				t.Errorf("Met(availability=%q) = %v, want %v", tt.availability, got, tt.want)
			}
		})
	}
}

// An unrecognized condition must never fire. This is what makes removing a
// condition type safe: leftover rows in the database evaluate to false rather
// than crashing or alerting spuriously.
func TestMet_UnknownConditionNeverFires(t *testing.T) {
	for _, typ := range []string{"price_below", "", "nonsense"} {
		if Met(Condition{Type: typ}, Observation{Availability: "onsale"}) {
			t.Errorf("Met(type=%q) = true, want false", typ)
		}
	}
}
