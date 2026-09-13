package notifier

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"ticket-watcher/internal/store/db"
)

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

func mustRender(t *testing.T, a Alert) Message {
	t.Helper()
	m, ok := render(a)
	if !ok {
		t.Fatalf("render returned ok=false for kind %q", a.Kind)
	}
	return m
}

func at(y int, mo time.Month, d, h int) *time.Time {
	t := time.Date(y, mo, d, h, 0, 0, 0, time.UTC)
	return &t
}

func TestRender_AddressesAndVenue(t *testing.T) {
	m := mustRender(t, Alert{
		EventName: "Show", Venue: "Arena", ToEmail: "u@e.com", Kind: "public_open",
	})
	if m.To != "u@e.com" {
		t.Errorf("To = %q", m.To)
	}
	if !strings.Contains(m.Subject, "Show") {
		t.Errorf("subject missing event name: %q", m.Subject)
	}
	if !strings.Contains(m.HTMLBody, "Arena") {
		t.Errorf("html missing venue: %q", m.HTMLBody)
	}
}

// Every milestone must produce a subject, a body, and an actionable link.
// An alert with no next step is a dead end for the reader.
func TestRender_EveryKindIsActionable(t *testing.T) {
	kinds := []string{
		"onsale_announced", "presale_open", "public_open",
		"status_onsale", "sale_closed", "cancelled", "rescheduled",
	}
	for _, k := range kinds {
		t.Run(k, func(t *testing.T) {
			m := mustRender(t, Alert{
				EventName: "Test Event", Kind: k, ToEmail: "u@e.com",
				EventURL:          "https://ticketmaster.com/e/1",
				PublicOnsaleStart: at(2026, time.September, 17, 15),
			})
			if m.Subject == "" {
				t.Error("empty subject")
			}
			if len(m.TextBody) < 40 {
				t.Errorf("text body too thin to be useful: %q", m.TextBody)
			}
			if !strings.Contains(m.TextBody, "https://ticketmaster.com/e/1") {
				t.Error("text body has no link — the reader has nowhere to go")
			}
			if !strings.Contains(m.HTMLBody, `href="https://ticketmaster.com/e/1"`) {
				t.Error("html body has no link")
			}
			// Emphasis markers must never leak to the reader.
			if strings.Contains(m.TextBody, "**") || strings.Contains(m.HTMLBody, "**") {
				t.Errorf("unconverted bold markers leaked: %q", m.TextBody)
			}
		})
	}
}

// The whole point of the sale_closed message: send the user to check resale,
// because we cannot see resale listings ourselves.
func TestRender_SaleClosedGuidesToResale(t *testing.T) {
	m := mustRender(t, Alert{
		EventName: "Show", Kind: "sale_closed", EventURL: "https://tm.test/e",
	})
	body := strings.ToLower(m.TextBody)
	if !strings.Contains(body, "resale") {
		t.Errorf("sale_closed must mention resale: %q", m.TextBody)
	}
	if !strings.Contains(strings.ToLower(m.HTMLBody), "resale") {
		t.Error("html body must mention resale too")
	}
	// The call-to-action should say what to do, not a generic "view event".
	if !strings.Contains(strings.ToLower(m.HTMLBody), "check for resale") {
		t.Errorf("expected a resale-specific link label: %q", m.HTMLBody)
	}
}

// A presale alert has to explain that access may be restricted, or users will
// click through, hit a code prompt, and think the app is broken.
func TestRender_PresaleExplainsAccess(t *testing.T) {
	m := mustRender(t, Alert{
		EventName: "Show", Kind: "presale_open", EventURL: "https://tm.test/e",
		EarliestPresaleName: "Artist", PublicOnsaleStart: at(2026, time.September, 20, 15),
	})
	body := strings.ToLower(m.TextBody)
	for _, want := range []string{"presale", "code"} {
		if !strings.Contains(body, want) {
			t.Errorf("presale copy missing %q: %s", want, m.TextBody)
		}
	}
	if !strings.Contains(m.TextBody, "20 Sep 2026") {
		t.Errorf("should tell the user when the public sale opens: %s", m.TextBody)
	}
}

// Sale-state messages must not claim tickets exist — we can't see resale.
func TestRender_DoesNotPromiseInventory(t *testing.T) {
	for _, k := range []string{"public_open", "status_onsale", "presale_open"} {
		m := mustRender(t, Alert{EventName: "Show", Kind: k, EventURL: "https://tm.test/e"})
		body := strings.ToLower(m.TextBody)
		for _, phrase := range []string{"seats available", "in stock", "guaranteed"} {
			if strings.Contains(body, phrase) {
				t.Errorf("%s overpromises (%q): %s", k, phrase, m.TextBody)
			}
		}
	}
}

// Event names come from the Ticketmaster API, so they must be escaped in HTML.
func TestRender_EscapesEventNameInHTML(t *testing.T) {
	m := mustRender(t, Alert{
		EventName: `Rock & Roll <script>alert(1)</script>`,
		Kind:      "public_open", EventURL: "https://tm.test/e",
	})
	if strings.Contains(m.HTMLBody, "<script>") {
		t.Errorf("unescaped markup from the event name reached the HTML body: %s", m.HTMLBody)
	}
	if !strings.Contains(m.HTMLBody, "&amp;") {
		t.Errorf("expected & to be escaped: %s", m.HTMLBody)
	}
	// The plain-text part should keep the original characters.
	if !strings.Contains(m.TextBody, "Rock & Roll") {
		t.Errorf("text body should not be HTML-escaped: %s", m.TextBody)
	}
}

func TestRender_UnknownKindIsNotSent(t *testing.T) {
	if _, ok := render(Alert{EventName: "Show", Kind: "no_such_kind"}); ok {
		t.Error("render returned ok=true for an unknown kind — that would mail an empty message")
	}
}

func TestNotify_FansOutAndLogs(t *testing.T) {
	s := &fakeSender{}
	st := &fakeStore{}
	New(st, s).Notify(context.Background(), Alert{
		WatchID: 7, ToEmail: "u@e.com", EventName: "Show", Kind: "public_open",
	})

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
	New(st, s).Notify(context.Background(), Alert{WatchID: 1, Kind: "public_open"})

	if len(st.logged) != 0 {
		t.Errorf("a failed send must not be recorded, got %d", len(st.logged))
	}
}

// An unknown kind must not reach a sender at all.
func TestNotify_UnknownKindSendsNothing(t *testing.T) {
	s := &fakeSender{}
	st := &fakeStore{}
	New(st, s).Notify(context.Background(), Alert{WatchID: 1, Kind: "bogus"})

	if len(s.sent) != 0 || len(st.logged) != 0 {
		t.Errorf("unknown kind produced sends=%d logs=%d, want 0/0", len(s.sent), len(st.logged))
	}
}

// The opt-out must survive the new rendering path.
func TestRender_CarriesUnsubscribe(t *testing.T) {
	m := mustRender(t, Alert{
		EventName: "Show", Kind: "public_open", EventURL: "https://tm.test/e",
		UnsubscribeURL: "https://app.test/api/unsubscribe?token=1.abc",
	})
	if !strings.Contains(m.TextBody, "unsubscribe?token=1.abc") {
		t.Error("text body missing the unsubscribe link")
	}
	if m.Headers["List-Unsubscribe"] == "" || m.Headers["List-Unsubscribe-Post"] == "" {
		t.Errorf("missing RFC 8058 headers: %+v", m.Headers)
	}
}

// Presale names from Ticketmaster are inconsistent — some already contain the
// word "presale". The copy must not read "The Presale presale".
func TestRender_PresaleNameNotDuplicated(t *testing.T) {
	cases := []struct{ name, wantIn, wantNotIn string }{
		{"Presale", "The Presale for", "Presale presale"},
		{"Artist Presale", "The Artist Presale for", "Presale presale"},
		{"Amex", "The Amex presale", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := mustRender(t, Alert{
				EventName: "Show", Kind: "presale_open", EventURL: "https://tm.test/e",
				EarliestPresaleName: c.name,
			})
			if !strings.Contains(m.TextBody, c.wantIn) {
				t.Errorf("body missing %q:\n%s", c.wantIn, m.TextBody)
			}
			if c.wantNotIn != "" && strings.Contains(m.TextBody, c.wantNotIn) {
				t.Errorf("body contains awkward %q:\n%s", c.wantNotIn, m.TextBody)
			}
		})
	}
}
