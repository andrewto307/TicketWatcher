// Package notifier delivers fired-condition alerts to one or more channels
// (email now, SMS later) and records each successful send in the notifications table.
package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"time"

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
	// Headers carries extra SMTP headers (List-Unsubscribe and friends). Empty
	// for transactional mail, which must not advertise an opt-out.
	Headers map[string]string
}

// Alert is the raw data a fired watch produces; the notifier renders it to a Message.
type Alert struct {
	WatchID   int64
	ToEmail   string
	EventName string
	Venue     string
	EventURL  string

	// Kind is the sale milestone being reported (evaluator.Kind). It selects the
	// copy and the call-to-action; see render.go.
	Kind         string
	Availability string

	// The sale calendar, used to tell the reader what happens next ("public sale
	// opens Thu 17 Sep at 15:00 UTC") rather than only what just happened.
	PublicOnsaleStart    *time.Time
	EarliestPresaleStart *time.Time
	EarliestPresaleName  string
	OnsaleTBD            bool

	// UnsubscribeURL opts the recipient out of all alerts. Required for every
	// alert we send; empty only in tests.
	UnsubscribeURL string
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
	msg, ok := render(a)
	if !ok {
		log.Printf("notifier: no copy for kind %q (watch %d) — not sending", a.Kind, a.WatchID)
		return
	}
	for _, s := range n.senders {
		if err := s.Send(ctx, msg); err != nil {
			log.Printf("notifier: %s send failed (watch %d): %v", s.Channel(), a.WatchID, err)
			continue
		}
		payload, _ := json.Marshal(map[string]any{
			"subject":      msg.Subject,
			"event":        a.EventName,
			"kind":         a.Kind,
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

// render builds the message for one milestone. ok=false when the kind has no
// copy, so the caller can decline to send rather than mail an empty body.
func render(a Alert) (Message, bool) {
	subject, lines, showResaleNote, ok := renderMilestone(a)
	if !ok {
		return Message{}, false
	}
	text, htmlBody := buildBodies(a, lines, showResaleNote)
	msg := Message{To: a.ToEmail, Subject: subject, HTMLBody: htmlBody, TextBody: text}

	// Every alert must carry a working opt-out: a visible footer link for the
	// reader, and List-Unsubscribe headers so Gmail/Outlook render their own
	// one-click unsubscribe button. Mailbox providers weigh that heavily —
	// without it, recipients reach for "mark as spam" instead, which is far
	// more damaging to the sending domain.
	if a.UnsubscribeURL != "" {
		msg.TextBody += fmt.Sprintf("\n\u2014\nStop receiving these alerts: %s\n", a.UnsubscribeURL)
		msg.HTMLBody += fmt.Sprintf(
			`<hr><p style="color:#666;font-size:12px">`+
				`You're receiving this because you set a watch on this event. `+
				`<a href="%s">Unsubscribe from all alerts</a>.</p>`,
			html.EscapeString(a.UnsubscribeURL))
		msg.Headers = map[string]string{
			"List-Unsubscribe":      "<" + a.UnsubscribeURL + ">",
			"List-Unsubscribe-Post": "List-Unsubscribe=One-Click", // RFC 8058
		}
	}
	return msg, true
}
