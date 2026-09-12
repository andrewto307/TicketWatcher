package worker

import (
	"testing"

	"ticket-watcher/internal/store/db"
)

func str(v string) *string { return &v }

func TestChanged(t *testing.T) {
	tests := []struct {
		name  string
		ev    db.Event
		avail string
		want  bool
	}{
		{
			name:  "first poll (nil last state) -> changed",
			ev:    db.Event{},
			avail: "onsale",
			want:  true,
		},
		{
			name:  "identical value -> not changed",
			ev:    db.Event{LastAvailability: str("onsale")},
			avail: "onsale",
			want:  false,
		},
		{
			name:  "went on sale -> changed",
			ev:    db.Event{LastAvailability: str("offsale")},
			avail: "onsale",
			want:  true,
		},
		{
			name:  "came off sale -> changed",
			ev:    db.Event{LastAvailability: str("onsale")},
			avail: "offsale",
			want:  true,
		},
		{
			name:  "became known (nil -> value) -> changed",
			ev:    db.Event{},
			avail: "unknown",
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := changed(tt.ev, tt.avail); got != tt.want {
				t.Errorf("changed() = %v, want %v", got, tt.want)
			}
		})
	}
}
