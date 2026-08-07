package service

import (
	"context"
	"encoding/json"
	"time"

	"ticket-watcher/internal/store/db"
)

// NotificationService lists a user's sent-alert log. The user id is supplied per
// call by the HTTP layer (from the authenticated token).
type NotificationService struct {
	q *db.Queries
}

func NewNotificationService(q *db.Queries) *NotificationService {
	return &NotificationService{q: q}
}

// NotificationView is the API representation of a sent notification.
type NotificationView struct {
	ID      int64           `json:"id"`
	WatchID int64           `json:"watch_id"`
	Channel string          `json:"channel"`
	SentAt  time.Time       `json:"sent_at"`
	Payload json.RawMessage `json:"payload"`
}

// List returns the user's notifications, newest first (capped).
func (s *NotificationService) List(ctx context.Context, userID int64, limit int32) ([]NotificationView, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.q.ListNotificationsForUser(ctx, db.ListNotificationsForUserParams{UserID: userID, Limit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]NotificationView, 0, len(rows))
	for _, n := range rows {
		out = append(out, NotificationView{
			ID:      n.ID,
			WatchID: n.WatchID,
			Channel: n.Channel,
			SentAt:  n.SentAt.Time,
			Payload: json.RawMessage(n.Payload),
		})
	}
	return out, nil
}
