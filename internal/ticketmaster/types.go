package ticketmaster

import "time"

// EventSnapshot is the normalized view of a Ticketmaster event that the rest of
// the application consumes. Price fields are pointers because the Discovery API
// does not always return a priceRanges block — nil means "price unknown".
type EventSnapshot struct {
	TMEventID    string     `json:"tm_event_id"`
	Name         string     `json:"name"`
	URL          string     `json:"url"`
	Venue        string     `json:"venue"`
	EventDate    *time.Time `json:"event_date"`
	MinPrice     *float64   `json:"min_price"`
	MaxPrice     *float64   `json:"max_price"`
	Availability string     `json:"availability"` // onsale | offsale | cancelled | ... | unknown
}

// --- internal wire types: only the fields we actually read from the API ---

type searchResponse struct {
	Embedded struct {
		Events []tmEvent `json:"events"`
	} `json:"_embedded"`
}

type tmEvent struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	URL         string         `json:"url"`
	Dates       tmDates        `json:"dates"`
	PriceRanges []tmPriceRange `json:"priceRanges"`
	Embedded    tmEmbedded     `json:"_embedded"`
}

type tmEmbedded struct {
	Venues []tmVenue `json:"venues"`
}

type tmVenue struct {
	Name string `json:"name"`
}

type tmPriceRange struct {
	Type string  `json:"type"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
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
