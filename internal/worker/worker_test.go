package worker

import (
	"testing"

	"ticket-watcher/internal/store/db"
)

func i64(v int64) *int64    { return &v }
func str(v string) *string  { return &v }

func TestChanged(t *testing.T) {
	tests := []struct {
		name  string
		ev    db.Event
		minC  *int64
		maxC  *int64
		avail string
		want  bool
	}{
		{
			name: "first poll (nil last state) -> changed",
			ev:   db.Event{},
			minC: i64(8950), maxC: i64(35000), avail: "onsale",
			want: true,
		},
		{
			name: "identical values -> not changed",
			ev:   db.Event{LastMinPriceCents: i64(8950), LastMaxPriceCents: i64(35000), LastAvailability: str("onsale")},
			minC: i64(8950), maxC: i64(35000), avail: "onsale",
			want: false,
		},
		{
			name: "min price drop -> changed",
			ev:   db.Event{LastMinPriceCents: i64(8950), LastMaxPriceCents: i64(35000), LastAvailability: str("onsale")},
			minC: i64(8000), maxC: i64(35000), avail: "onsale",
			want: true,
		},
		{
			name: "availability change -> changed",
			ev:   db.Event{LastMinPriceCents: i64(8950), LastMaxPriceCents: i64(35000), LastAvailability: str("offsale")},
			minC: i64(8950), maxC: i64(35000), avail: "onsale",
			want: true,
		},
		{
			name: "price becomes known (nil -> value) -> changed",
			ev:   db.Event{LastAvailability: str("onsale")},
			minC: i64(5000), maxC: nil, avail: "onsale",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := changed(tt.ev, tt.minC, tt.maxC, tt.avail); got != tt.want {
				t.Errorf("changed() = %v, want %v", got, tt.want)
			}
		})
	}
}
