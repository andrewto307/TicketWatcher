package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"ticket-watcher/internal/money"
	"ticket-watcher/internal/store"
	"ticket-watcher/internal/store/db"
	"ticket-watcher/internal/ticketmaster"
)

var (
	ErrInvalidCondition  = errors.New("condition_type must be 'price_below' or 'becomes_available'")
	ErrThresholdRequired = errors.New("threshold is required for condition_type 'price_below'")
	ErrMissingEventID    = errors.New("tm_event_id is required")
)

// Fetcher is the slice of the Ticketmaster client the watch service needs.
type Fetcher interface {
	GetEvent(ctx context.Context, tmEventID string) (ticketmaster.EventSnapshot, error)
}

// WatchService creates and lists a user's watches.
type WatchService struct {
	q      *db.Queries
	tm     Fetcher
	userID int64 // hardcoded demo user in v1 (auth arrives in Phase 5)
}

func NewWatchService(q *db.Queries, tm Fetcher, userID int64) *WatchService {
	return &WatchService{q: q, tm: tm, userID: userID}
}

// CreateWatchInput is the validated input for creating a watch (dollars).
type CreateWatchInput struct {
	TMEventID     string
	ConditionType string
	Threshold     *float64 // required for price_below
	PollIntervalS int32    // <= 0 -> default 300
}

// WatchView is the API representation of a watch: dollars, flattened event info.
type WatchView struct {
	ID            int64      `json:"id"`
	TMEventID     string     `json:"tm_event_id"`
	EventName     string     `json:"event_name"`
	Venue         string     `json:"venue"`
	EventDate     *time.Time `json:"event_date"`
	ConditionType string     `json:"condition_type"`
	Threshold     *float64   `json:"threshold"`
	Status        string     `json:"status"`
	CurrentMin    *float64   `json:"current_min_price"`
	CurrentMax    *float64   `json:"current_max_price"`
	Availability  *string    `json:"availability"`
	PollIntervalS int32      `json:"poll_interval_s"`
	CreatedAt     time.Time  `json:"created_at"`
}

// Create validates the input, ensures the event exists (fetching from Ticketmaster
// only for events we've never seen — this dedupes polling), and creates the watch.
func (s *WatchService) Create(ctx context.Context, in CreateWatchInput) (WatchView, error) {
	if in.TMEventID == "" {
		return WatchView{}, ErrMissingEventID
	}
	if in.ConditionType != "price_below" && in.ConditionType != "becomes_available" {
		return WatchView{}, ErrInvalidCondition
	}
	if in.ConditionType == "price_below" && in.Threshold == nil {
		return WatchView{}, ErrThresholdRequired
	}

	ev, err := s.q.GetEventByTMID(ctx, in.TMEventID)
	if errors.Is(err, pgx.ErrNoRows) {
		snap, ferr := s.tm.GetEvent(ctx, in.TMEventID) // one API call, only for new events
		if ferr != nil {
			return WatchView{}, fmt.Errorf("fetch event %q: %w", in.TMEventID, ferr)
		}
		ev, err = s.q.UpsertEventByTMID(ctx, db.UpsertEventByTMIDParams{
			TmEventID: snap.TMEventID,
			Name:      snap.Name,
			Url:       snap.URL,
			Venue:     snap.Venue,
			EventDate: store.TSPtr(snap.EventDate),
		})
	}
	if err != nil {
		return WatchView{}, fmt.Errorf("resolve event: %w", err)
	}

	interval := in.PollIntervalS
	if interval <= 0 {
		interval = 300
	}
	w, err := s.q.CreateWatch(ctx, db.CreateWatchParams{
		UserID:         s.userID,
		EventID:        ev.ID,
		ConditionType:  in.ConditionType,
		ThresholdCents: money.ToCents(in.Threshold),
		PollIntervalS:  interval,
	})
	if err != nil {
		return WatchView{}, fmt.Errorf("create watch: %w", err)
	}
	return watchViewFromEvent(w, ev), nil
}

// List returns all of the user's watches with their latest event state.
func (s *WatchService) List(ctx context.Context) ([]WatchView, error) {
	rows, err := s.q.ListWatchesWithEvent(ctx, s.userID)
	if err != nil {
		return nil, err
	}
	out := make([]WatchView, 0, len(rows))
	for _, r := range rows {
		out = append(out, watchViewFromRow(r))
	}
	return out, nil
}

func watchViewFromEvent(w db.Watch, ev db.Event) WatchView {
	return WatchView{
		ID:            w.ID,
		TMEventID:     ev.TmEventID,
		EventName:     ev.Name,
		Venue:         ev.Venue,
		EventDate:     store.TimePtr(ev.EventDate),
		ConditionType: w.ConditionType,
		Threshold:     money.ToDollars(w.ThresholdCents),
		Status:        w.Status,
		CurrentMin:    money.ToDollars(ev.LastMinPriceCents),
		CurrentMax:    money.ToDollars(ev.LastMaxPriceCents),
		Availability:  ev.LastAvailability,
		PollIntervalS: w.PollIntervalS,
		CreatedAt:     w.CreatedAt.Time,
	}
}

func watchViewFromRow(r db.ListWatchesWithEventRow) WatchView {
	return WatchView{
		ID:            r.ID,
		TMEventID:     r.TmEventID,
		EventName:     r.EventName,
		Venue:         r.Venue,
		EventDate:     store.TimePtr(r.EventDate),
		ConditionType: r.ConditionType,
		Threshold:     money.ToDollars(r.ThresholdCents),
		Status:        r.Status,
		CurrentMin:    money.ToDollars(r.LastMinPriceCents),
		CurrentMax:    money.ToDollars(r.LastMaxPriceCents),
		Availability:  r.LastAvailability,
		PollIntervalS: r.PollIntervalS,
		CreatedAt:     r.CreatedAt.Time,
	}
}
