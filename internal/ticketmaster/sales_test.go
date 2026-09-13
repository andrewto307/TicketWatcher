package ticketmaster

import (
	"testing"
	"time"
)

// Shapes captured from live Discovery responses (2026-09-13).
const salesFixture = `{
  "_embedded": { "events": [
    {
      "id": "E-FULL", "name": "Full Sale Calendar",
      "dates": { "start": { "dateTime": "2026-12-30T01:00:00Z" }, "status": { "code": "offsale" } },
      "sales": {
        "public": { "startDateTime": "2026-09-17T15:00:00Z", "endDateTime": "2026-12-31T03:00:00Z",
                    "startTBD": false, "startTBA": false },
        "presales": [
          { "startDateTime": "2026-09-16T15:00:00Z", "endDateTime": "2026-09-17T02:00:00Z", "name": "Team Presale" },
          { "startDateTime": "2026-09-15T15:00:00Z", "endDateTime": "2026-09-16T02:00:00Z", "name": "STH Presale" },
          { "startDateTime": "2026-09-15T15:00:00Z", "endDateTime": "2026-09-17T02:00:00Z", "name": "Amex Presale" }
        ]
      }
    },
    {
      "id": "E-TBD", "name": "Onsale Date Not Announced",
      "dates": { "start": { "localDate": "2027-03-01" }, "status": { "code": "offsale" } },
      "sales": {
        "public": { "startDateTime": "9999-12-31T06:00:00Z", "endDateTime": "2027-03-02T03:30:00Z",
                    "startTBD": false, "startTBA": false }
      }
    },
    {
      "id": "E-BARE", "name": "No Sales Block At All",
      "dates": { "start": { "dateTime": "2026-11-01T20:00:00Z" }, "status": { "code": "onsale" } }
    }
  ]}
}`

func snapshots(t *testing.T) map[string]EventSnapshot {
	t.Helper()
	evs, err := parseSearchResponse([]byte(salesFixture))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := map[string]EventSnapshot{}
	for _, e := range evs {
		out[e.TMEventID] = e
	}
	return out
}

func TestParseSales_FullCalendar(t *testing.T) {
	s := snapshots(t)["E-FULL"]

	if s.PublicOnsaleStart == nil {
		t.Fatal("PublicOnsaleStart = nil, want parsed date")
	}
	if got, want := s.PublicOnsaleStart.UTC(), time.Date(2026, 9, 17, 15, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("PublicOnsaleStart = %v, want %v", got, want)
	}
	if s.PublicOnsaleEnd == nil || !s.PublicOnsaleEnd.UTC().Equal(time.Date(2026, 12, 31, 3, 0, 0, 0, time.UTC)) {
		t.Errorf("PublicOnsaleEnd = %v", s.PublicOnsaleEnd)
	}
	if s.OnsaleTBD {
		t.Error("OnsaleTBD = true, want false (a real date was given)")
	}
	if s.PresaleCount != 3 {
		t.Errorf("PresaleCount = %d, want 3", s.PresaleCount)
	}

	// Two presales tie at the earliest instant; the first such one encountered wins,
	// and either name is acceptable — what matters is the time and that it's one of them.
	if s.EarliestPresaleStart == nil {
		t.Fatal("EarliestPresaleStart = nil")
	}
	if got, want := s.EarliestPresaleStart.UTC(), time.Date(2026, 9, 15, 15, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("EarliestPresaleStart = %v, want %v (the earliest, not the first listed)", got, want)
	}
	if s.EarliestPresaleName != "STH Presale" && s.EarliestPresaleName != "Amex Presale" {
		t.Errorf("EarliestPresaleName = %q, want one of the two earliest windows", s.EarliestPresaleName)
	}
}

// The 9999 sentinel must not become a real date: every downstream comparison
// ("has the sale started?") would answer wrongly for the next 8,000 years.
func TestParseSales_UnannouncedOnsaleIsTBDNotADate(t *testing.T) {
	s := snapshots(t)["E-TBD"]

	if !s.OnsaleTBD {
		t.Error("OnsaleTBD = false, want true for the 9999-12-31 sentinel")
	}
	if s.PublicOnsaleStart != nil {
		t.Errorf("PublicOnsaleStart = %v, want nil for a sentinel date", s.PublicOnsaleStart)
	}
	// The end date on such events is real and still usable.
	if s.PublicOnsaleEnd == nil {
		t.Error("PublicOnsaleEnd = nil, want the real end date to survive")
	}
	if s.PresaleCount != 0 {
		t.Errorf("PresaleCount = %d, want 0", s.PresaleCount)
	}
}

// A missing sales block must parse cleanly rather than panic or invent dates.
func TestParseSales_AbsentBlock(t *testing.T) {
	s := snapshots(t)["E-BARE"]

	if s.PublicOnsaleStart != nil || s.PublicOnsaleEnd != nil || s.EarliestPresaleStart != nil {
		t.Errorf("expected all sale dates nil, got start=%v end=%v presale=%v",
			s.PublicOnsaleStart, s.PublicOnsaleEnd, s.EarliestPresaleStart)
	}
	if s.OnsaleTBD {
		t.Error("OnsaleTBD = true, want false — absent is not the same as 'announced later'")
	}
	if s.Availability != "onsale" {
		t.Errorf("Availability = %q, want onsale", s.Availability)
	}
}

func TestParseSaleTime(t *testing.T) {
	tests := []struct {
		name, in string
		wantNil  bool
		wantTBD  bool
	}{
		{"empty", "", true, false},
		{"garbage", "not-a-date", true, false},
		{"real date", "2026-09-17T15:00:00Z", false, false},
		{"sentinel 9999", "9999-12-31T06:00:00Z", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, tbd := parseSaleTime(tt.in)
			if (got == nil) != tt.wantNil {
				t.Errorf("time nil = %v, want %v", got == nil, tt.wantNil)
			}
			if tbd != tt.wantTBD {
				t.Errorf("tbd = %v, want %v", tbd, tt.wantTBD)
			}
		})
	}
}
