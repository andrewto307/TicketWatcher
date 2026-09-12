package ticketmaster

import "time"

// EventSnapshot is the normalized view of a Ticketmaster event that the rest of
// the application consumes.
//
// There are no price fields: Ticketmaster removed priceRanges from the Discovery
// API on 2025-03-11 and it now always returns null. See plan/06-design-decisions.md
// D13.
type EventSnapshot struct {
	TMEventID    string     `json:"tm_event_id"`
	Name         string     `json:"name"`
	URL          string     `json:"url"`
	Venue        string     `json:"venue"`
	EventDate    *time.Time `json:"event_date"`
	Availability string     `json:"availability"` // onsale | offsale | cancelled | postponed | rescheduled | unknown
}

// --- internal wire types: only the fields we actually read from the API ---

type searchResponse struct {
	Embedded struct {
		Events []tmEvent `json:"events"`
	} `json:"_embedded"`
}

type tmEvent struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	URL      string     `json:"url"`
	Dates    tmDates    `json:"dates"`
	Embedded tmEmbedded `json:"_embedded"`
}

type tmEmbedded struct {
	Venues []tmVenue `json:"venues"`
}

type tmVenue struct {
	Name string `json:"name"`
}

type tmDates struct {
	Start  tmStart  `json:"start"`
	Status tmStatus `json:"status"`
}

type tmStart struct {
	DateTime  string `json:"dateTime"`  // RFC3339; may be absent
	LocalDate string `json:"localDate"` // fallback; date only, no time
}

type tmStatus struct {
	Code string `json:"code"`
}
