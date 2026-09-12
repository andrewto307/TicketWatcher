package ticketmaster

import "testing"

// A captured-shape Discovery search payload with two events:
//   1. full data (venue, dateTime, onsale)
//   2. only a localDate (nil date) and offsale
//
// The fixture deliberately keeps the `priceRanges` block that real responses used
// to carry: Ticketmaster removed it in 2025 (D13) and we no longer parse it, but
// an unknown field must not break decoding if it ever reappears.
const searchFixture = `{
  "_embedded": {
    "events": [
      {
        "name": "Radiohead",
        "id": "G5vYZ9abc",
        "url": "https://www.ticketmaster.com/event/G5vYZ9abc",
        "dates": {
          "start": { "dateTime": "2026-09-01T20:00:00Z" },
          "status": { "code": "onsale" }
        },
        "priceRanges": [
          { "type": "standard", "currency": "USD", "min": 89.5, "max": 350 }
        ],
        "_embedded": { "venues": [ { "name": "Madison Square Garden" } ] }
      },
      {
        "name": "Local Indie Show",
        "id": "G5vABC123",
        "url": "https://www.ticketmaster.com/event/G5vABC123",
        "dates": {
          "start": { "localDate": "2026-10-15" },
          "status": { "code": "offsale" }
        },
        "_embedded": { "venues": [ { "name": "The Basement" } ] }
      }
    ]
  }
}`

func TestParseSearchResponse(t *testing.T) {
	events, err := parseSearchResponse([]byte(searchFixture))
	if err != nil {
		t.Fatalf("parseSearchResponse: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Event 1: fully populated.
	rh := events[0]
	if rh.Name != "Radiohead" {
		t.Errorf("name = %q, want Radiohead", rh.Name)
	}
	if rh.Venue != "Madison Square Garden" {
		t.Errorf("venue = %q", rh.Venue)
	}
	if rh.Availability != "onsale" {
		t.Errorf("availability = %q, want onsale", rh.Availability)
	}
	if rh.EventDate == nil {
		t.Error("event date = nil, want parsed time")
	}

	// Event 2: only localDate -> nil date.
	indie := events[1]
	if indie.EventDate != nil {
		t.Errorf("expected nil event date (only localDate present), got %v", indie.EventDate)
	}
	if indie.Availability != "offsale" {
		t.Errorf("availability = %q, want offsale", indie.Availability)
	}
	if indie.Venue != "The Basement" {
		t.Errorf("venue = %q", indie.Venue)
	}
}

func TestNormalizeStatus(t *testing.T) {
	if got := normalizeStatus(""); got != "unknown" {
		t.Errorf("normalizeStatus(\"\") = %q, want unknown", got)
	}
	if got := normalizeStatus("onsale"); got != "onsale" {
		t.Errorf("normalizeStatus(onsale) = %q", got)
	}
}
