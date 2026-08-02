package money

import "testing"

func approx(a, b float64) bool { d := a - b; return d < 1e-9 && d > -1e-9 }

func TestToCents(t *testing.T) {
	if ToCents(nil) != nil {
		t.Error("ToCents(nil) should be nil")
	}
	cases := map[float64]int64{89.5: 8950, 28.69: 2869, 0: 0, 199.99: 19999, 100: 10000}
	for dollars, wantCents := range cases {
		d := dollars
		got := ToCents(&d)
		if got == nil || *got != wantCents {
			t.Errorf("ToCents(%v) = %v, want %d", dollars, got, wantCents)
		}
	}
}

func TestToDollars(t *testing.T) {
	if ToDollars(nil) != nil {
		t.Error("ToDollars(nil) should be nil")
	}
	c := int64(2869)
	if got := ToDollars(&c); got == nil || !approx(*got, 28.69) {
		t.Errorf("ToDollars(2869) = %v, want 28.69", got)
	}
}

func TestRoundTrip(t *testing.T) {
	for _, dollars := range []float64{0, 10, 28.69, 199.99, 1234.56} {
		d := dollars
		if back := ToDollars(ToCents(&d)); back == nil || !approx(*back, dollars) {
			t.Errorf("round trip %v -> %v", dollars, back)
		}
	}
}
