package notifier

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ticket-watcher/internal/store/db"
)

func cents(v int64) *int64 { return &v }

type fakeSender struct {
	sent []Message
	err  error
}

func (f *fakeSender) Channel() string { return "email" }
func (f *fakeSender) Send(_ context.Context, m Message) error {
	f.sent = append(f.sent, m)
	return f.err
}

type fakeStore struct{ logged []db.InsertNotificationParams }

func (f *fakeStore) InsertNotification(_ context.Context, arg db.InsertNotificationParams) (db.Notification, error) {
	f.logged = append(f.logged, arg)
	return db.Notification{ID: int64(len(f.logged))}, nil
}

func TestRender_PriceBelow(t *testing.T) {
	m := render(Alert{
		EventName: "Show", Venue: "Arena", ToEmail: "u@e.com",
		ConditionType: "price_below", MinPriceCents: cents(2869), ThresholdCents: cents(10000),
	})
	if m.To != "u@e.com" {
		t.Errorf("To = %q", m.To)
	}
	if !strings.Contains(m.Subject, "$28.69") {
		t.Errorf("subject missing price: %q", m.Subject)
	}
	if !strings.Contains(m.HTMLBody, "Arena") {
		t.Errorf("html missing venue: %q", m.HTMLBody)
	}
}

func TestRender_BecomesAvailable(t *testing.T) {
	m := render(Alert{EventName: "Show", ConditionType: "becomes_available"})
	if !strings.Contains(strings.ToLower(m.Subject), "on sale") {
		t.Errorf("subject = %q, want 'on sale'", m.Subject)
	}
}

func TestNotify_FansOutAndLogs(t *testing.T) {
	s := &fakeSender{}
	st := &fakeStore{}
	New(st, s).Notify(context.Background(), Alert{WatchID: 7, ToEmail: "u@e.com", EventName: "Show", ConditionType: "becomes_available"})

	if len(s.sent) != 1 {
		t.Fatalf("sender called %d times, want 1", len(s.sent))
	}
	if len(st.logged) != 1 {
		t.Fatalf("logged %d notifications, want 1", len(st.logged))
	}
	if st.logged[0].WatchID != 7 || st.logged[0].Channel != "email" {
		t.Errorf("logged = %+v", st.logged[0])
	}
}

func TestNotify_FailedSendIsNotLogged(t *testing.T) {
	s := &fakeSender{err: errors.New("boom")}
	st := &fakeStore{}
	New(st, s).Notify(context.Background(), Alert{WatchID: 1, ConditionType: "becomes_available"})

	if len(st.logged) != 0 {
		t.Errorf("a failed send must not be recorded, got %d", len(st.logged))
	}
}
