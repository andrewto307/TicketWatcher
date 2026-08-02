// Package notifier delivers fired-condition alerts to one or more channels
// (email now, SMS later) and records each successful send in the notifications table.
package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"ticket-watcher/internal/money"
	"ticket-watcher/internal/store/db"
)

// Sender delivers a rendered message over a single channel.
type Sender interface {
	Send(ctx context.Context, msg Message) error
	Channel() string // "email" | "sms"
}

// Message is a rendered, channel-agnostic alert.
type Message struct {
	To       string
	Subject  string
	HTMLBody string
	TextBody string
}

// Alert is the raw data a fired watch produces; the notifier renders it to a Message.
type Alert struct {
	WatchID        int64
	ToEmail        string
	EventName      string
	Venue          string
	EventURL       string
	ConditionType  string
	ThresholdCents *int64
	MinPriceCents  *int64
	Availability   string
}

// Store is the slice of the database the notifier needs.
type Store interface {
	InsertNotification(ctx context.Context, arg db.InsertNotificationParams) (db.Notification, error)
}

// Notifier fans an alert out to every sender and records each successful send.
type Notifier struct {
	senders []Sender
	store   Store
}

func New(store Store, senders ...Sender) *Notifier {
	return &Notifier{senders: senders, store: store}
}

// Notify renders the alert, sends it on every channel, and records successes.
// Errors are logged, never returned: a failing channel must not stall the poll loop.
func (n *Notifier) Notify(ctx context.Context, a Alert) {
	msg := render(a)
	for _, s := range n.senders {
		if err := s.Send(ctx, msg); err != nil {
			log.Printf("notifier: %s send failed (watch %d): %v", s.Channel(), a.WatchID, err)
			continue
		}
		payload, _ := json.Marshal(map[string]any{
			"subject":      msg.Subject,
			"event":        a.EventName,
			"condition":    a.ConditionType,
			"min_price":    money.ToDollars(a.MinPriceCents),
			"threshold":    money.ToDollars(a.ThresholdCents),
			"availability": a.Availability,
		})
		if _, err := n.store.InsertNotification(ctx, db.InsertNotificationParams{
			WatchID: a.WatchID,
			Channel: s.Channel(),
			Payload: payload,
		}); err != nil {
			log.Printf("notifier: record %s notification failed (watch %d): %v", s.Channel(), a.WatchID, err)
		}
	}
}

// render builds a human-friendly subject/body from an alert.
func render(a Alert) Message {
	var subject, line string
	switch a.ConditionType {
	case "price_below":
		price, thr := dollars(a.MinPriceCents), dollars(a.ThresholdCents)
		subject = fmt.Sprintf("🎟️ Price drop: %s is now %s", a.EventName, price)
		line = fmt.Sprintf("The minimum price just dropped to %s, below your %s threshold.", price, thr)
	case "becomes_available":
		subject = fmt.Sprintf("🎟️ %s is on sale", a.EventName)
		line = "This event just became available."
	default:
		subject = fmt.Sprintf("🎟️ Update: %s", a.EventName)
		line = "Your watch triggered."
	}

	venue := ""
	if a.Venue != "" {
		venue = " @ " + a.Venue
	}
	text := fmt.Sprintf("%s%s\n\n%s\n\n%s", a.EventName, venue, line, a.EventURL)
	html := fmt.Sprintf(`<h2>%s%s</h2><p>%s</p><p><a href="%s">View on Ticketmaster →</a></p>`,
		a.EventName, venue, line, a.EventURL)
	return Message{To: a.ToEmail, Subject: subject, HTMLBody: html, TextBody: text}
}

func dollars(cents *int64) string {
	if d := money.ToDollars(cents); d != nil {
		return fmt.Sprintf("$%.2f", *d)
	}
	return "an unknown price"
}
