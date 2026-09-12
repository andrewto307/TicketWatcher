package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"ticket-watcher/internal/store"
	"ticket-watcher/internal/store/db"
	"ticket-watcher/internal/ticketmaster"
)

var (
	ErrInvalidCondition  = errors.New("condition_type must be 'becomes_available'")
	ErrMissingEventID    = errors.New("tm_event_id is required")
	ErrWatchNotFound     = errors.New("watch not found")
	ErrInvalidStatus     = errors.New("status must be 'active' or 'paused'")
	ErrWatchLimitReached = errors.New("watch limit reached")
)

// Fetcher is the slice of the Ticketmaster client the watch service needs.
type Fetcher interface {
	GetEvent(ctx context.Context, tmEventID string) (ticketmaster.EventSnapshot, error)
}

// WatchService creates and manages a user's watches. The user id is supplied per
// call by the HTTP layer (from the authenticated token), not held on the service.
type WatchService struct {
	q          *db.Queries
	tm         Fetcher
	maxPerUser int // 0 = unlimited
}

// NewWatchService caps each user at maxPerUser watches. The cap exists because
// every watched event draws on one shared Ticketmaster budget (5,000 req/day) —
// without it a single account could starve everyone else. Pass 0 to disable.
func NewWatchService(q *db.Queries, tm Fetcher, maxPerUser int) *WatchService {
	return &WatchService{q: q, tm: tm, maxPerUser: maxPerUser}
}

// CreateWatchInput is the validated input for creating a watch.
type CreateWatchInput struct {
	TMEventID     string
	ConditionType string // "becomes_available" (the only condition; see D13)
	PollIntervalS int32  // <= 0 -> default 300
}

// WatchView is the API representation of a watch, with flattened event info.
type WatchView struct {
	ID            int64      `json:"id"`
	TMEventID     string     `json:"tm_event_id"`
	EventName     string     `json:"event_name"`
	Venue         string     `json:"venue"`
	EventDate     *time.Time `json:"event_date"`
	ConditionType string     `json:"condition_type"`
	Status        string     `json:"status"`
	Availability  *string    `json:"availability"`
	PollIntervalS int32      `json:"poll_interval_s"`
	CreatedAt     time.Time  `json:"created_at"`
	// LastPolledAt lets the UI say "checking…" before the first poll rather than
	// showing an empty status the user can't interpret.
	LastPolledAt *time.Time `json:"last_polled_at"`
}

// SnapshotView is one point of an event's availability history.
type SnapshotView struct {
	Availability *string   `json:"availability"`
	CheckedAt    time.Time `json:"checked_at"`
}

// UpdateWatchInput is a partial update; nil fields are left unchanged.
type UpdateWatchInput struct {
	Status *string // "active" | "paused"
}

// Create validates the input, ensures the event exists (fetching from Ticketmaster
// only for events we've never seen — this dedupes polling), creates the watch, and
// marks the event due so the new watch is evaluated on the next tick.
func (s *WatchService) Create(ctx context.Context, userID int64, in CreateWatchInput) (WatchView, error) {
	if in.TMEventID == "" {
		return WatchView{}, ErrMissingEventID
	}
	if in.ConditionType != "becomes_available" {
		return WatchView{}, ErrInvalidCondition
	}

	// Check the cap before resolving the event: an over-limit request must not
	// spend a Ticketmaster call on an event we're about to refuse to watch.
	if s.maxPerUser > 0 {
		n, err := s.q.CountWatchesForUser(ctx, userID)
		if err != nil {
			return WatchView{}, fmt.Errorf("count watches: %w", err)
		}
		if n >= int64(s.maxPerUser) {
			return WatchView{}, fmt.Errorf("%w: you can track up to %d events at once — remove one to add another",
				ErrWatchLimitReached, s.maxPerUser)
		}
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
		UserID:        userID,
		EventID:       ev.ID,
		ConditionType: in.ConditionType,
		PollIntervalS: interval,
	})
	if err != nil {
		return WatchView{}, fmt.Errorf("create watch: %w", err)
	}

	// Make the event due now so the scheduler evaluates this new watch on its next
	// tick (~15s), instead of waiting for the event's next scheduled poll (which can
	// be up to poll_interval_s away if the event was already being tracked).
	// Best-effort: a failure here only delays the first evaluation, so don't fail the create.
	_ = s.q.MarkEventDue(ctx, ev.ID)

	return watchViewFromEvent(w, ev), nil
}

// List returns all of the user's watches with their latest event state.
func (s *WatchService) List(ctx context.Context, userID int64) ([]WatchView, error) {
	rows, err := s.q.ListWatchesWithEvent(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]WatchView, 0, len(rows))
	for _, r := range rows {
		out = append(out, watchViewFromRow(r))
	}
	return out, nil
}

// History returns the availability snapshots for a watch's event.
func (s *WatchService) History(ctx context.Context, userID, watchID int64) ([]SnapshotView, error) {
	w, err := s.q.GetWatch(ctx, db.GetWatchParams{ID: watchID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWatchNotFound
	}
	if err != nil {
		return nil, err
	}
	snaps, err := s.q.ListSnapshotsForEvent(ctx, w.EventID)
	if err != nil {
		return nil, err
	}
	out := make([]SnapshotView, 0, len(snaps))
	for _, sp := range snaps {
		out = append(out, SnapshotView{
			Availability: sp.AvailabilityStatus,
			CheckedAt:    sp.CheckedAt.Time,
		})
	}
	return out, nil
}

// Update pauses or resumes a watch. Resuming re-arms it (resets last_evaluation)
// so a condition that is already true re-establishes its edge and can fire again.
func (s *WatchService) Update(ctx context.Context, userID, watchID int64, in UpdateWatchInput) (WatchView, error) {
	if in.Status != nil && *in.Status != "active" && *in.Status != "paused" {
		return WatchView{}, ErrInvalidStatus
	}
	w, err := s.q.UpdateWatch(ctx, db.UpdateWatchParams{
		ID:              watchID,
		UserID:          userID,
		Status:          in.Status,
		ResetEvaluation: in.Status != nil && *in.Status == "active",
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return WatchView{}, ErrWatchNotFound
	}
	if err != nil {
		return WatchView{}, err
	}
	ev, err := s.q.GetEvent(ctx, w.EventID)
	if err != nil {
		return WatchView{}, err
	}
	return watchViewFromEvent(w, ev), nil
}

// Delete removes a watch owned by the user.
func (s *WatchService) Delete(ctx context.Context, userID, watchID int64) error {
	n, err := s.q.DeleteWatch(ctx, db.DeleteWatchParams{ID: watchID, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrWatchNotFound
	}
	return nil
}

func watchViewFromEvent(w db.Watch, ev db.Event) WatchView {
	return WatchView{
		ID:            w.ID,
		TMEventID:     ev.TmEventID,
		EventName:     ev.Name,
		Venue:         ev.Venue,
		EventDate:     store.TimePtr(ev.EventDate),
		ConditionType: w.ConditionType,
		Status:        w.Status,
		Availability:  ev.LastAvailability,
		PollIntervalS: w.PollIntervalS,
		CreatedAt:     w.CreatedAt.Time,
		LastPolledAt:  store.TimePtr(ev.LastPolledAt),
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
		Status:        r.Status,
		Availability:  r.LastAvailability,
		PollIntervalS: r.PollIntervalS,
		CreatedAt:     r.CreatedAt.Time,
		LastPolledAt:  store.TimePtr(r.LastPolledAt),
	}
}
