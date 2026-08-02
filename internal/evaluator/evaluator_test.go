package evaluator

import "testing"

func cents(v int64) *int64 { return &v }

func TestMet_PriceBelow(t *testing.T) {
	tests := []struct {
		name      string
		threshold *int64
		minPrice  *int64
		want      bool
	}{
		{"below threshold -> met", cents(20000), cents(18000), true},
		{"above threshold -> not met", cents(20000), cents(25000), false},
		{"equal -> not met (strictly below)", cents(20000), cents(20000), false},
		{"price unknown -> not met", cents(20000), nil, false},
		{"no threshold -> not met", nil, cents(18000), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Met(
				Condition{Type: "price_below", ThresholdCents: tt.threshold},
				Observation{MinPriceCents: tt.minPrice},
			)
			if got != tt.want {
				t.Errorf("Met = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMet_BecomesAvailable(t *testing.T) {
	if !Met(Condition{Type: "becomes_available"}, Observation{Availability: "onsale"}) {
		t.Error("onsale should be met")
	}
	for _, s := range []string{"offsale", "cancelled", "unknown", ""} {
		if Met(Condition{Type: "becomes_available"}, Observation{Availability: s}) {
			t.Errorf("%q should not be met", s)
		}
	}
}

func TestMet_UnknownCondition(t *testing.T) {
	if Met(Condition{Type: "bogus"}, Observation{Availability: "onsale", MinPriceCents: cents(1)}) {
		t.Error("unknown condition type should never be met")
	}
}
