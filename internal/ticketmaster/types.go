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

	// --- the sale calendar, from the API's `sales` block ---
	// See plan/08-sale-milestone-alerts.md. Status alone flips only when the
	// *public* sale opens, so these dates are what make presale and
	// "date announced" alerts possible.

	// PublicOnsaleStart is nil when unknown. OnsaleTBD distinguishes "Ticketmaster
	// hasn't announced a date" from "the field was absent".
	PublicOnsaleStart *time.Time `json:"public_onsale_start"`
	PublicOnsaleEnd   *time.Time `json:"public_onsale_end"`
	OnsaleTBD         bool       `json:"onsale_tbd"`

	// Earliest presale window, and how many there are. Only the earliest triggers
	// an alert (some events carry a dozen); the count is for display.
	EarliestPresaleStart *time.Time `json:"earliest_presale_start"`
	EarliestPresaleName  string     `json:"earliest_presale_name"`
	PresaleCount         int        `json:"presale_count"`
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
	Sales    tmSales    `json:"sales"`
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

type tmSales struct {
	Public   tmPublicSale `json:"public"`
	Presales []tmPresale  `json:"presales"`
}

type tmPublicSale struct {
	StartDateTime string `json:"startDateTime"` // may be the 9999-12-31 sentinel
	EndDateTime   string `json:"endDateTime"`
	StartTBD      bool   `json:"startTBD"`
	StartTBA      bool   `json:"startTBA"`
}

type tmPresale struct {
	StartDateTime string `json:"startDateTime"`
	EndDateTime   string `json:"endDateTime"`
	Name          string `json:"name"`
}
