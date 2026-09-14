//go:build integration

package inttest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"ticket-watcher/internal/auth"
	"ticket-watcher/internal/notifier"
	"ticket-watcher/internal/service"
	"ticket-watcher/internal/store/db"
	"ticket-watcher/internal/ticketmaster"
	"ticket-watcher/internal/worker"
)

// A scenario harness that runs the REAL code path end to end:
//
//	WatchService.Create -> Postgres -> worker.ProcessEvent -> evaluator -> notifier
//
// Only two things are substituted: the Ticketmaster HTTP response (so a test can
// make time appear to pass) and the email sender (so sends can be asserted).
// Everything else — the SQL, the service, the worker, the evaluator, the message
// rendering — is production code.
//
// This layer exists because both bugs found in manual testing lived *between*
// components, where unit tests with hand-written fakes are blind by construction:
//
//   - "watch creation discards the snapshot" was in the service/SQL boundary; the
//     worker's fakeStore has no UpsertEventByTMID at all.
//   - "last_evaluation seeds wrong" passed every evaluator unit test, because the
//     test supplied a correct Prior by hand while production built a wrong one.
//
// Anything asserted here is asserted against what the application actually does.

// --- programmable Ticketmaster stub ---

type tmStub struct {
	mu      sync.Mutex
	payload string
	calls   int
}

func (s *tmStub) set(payload string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payload = payload
}

func (s *tmStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *tmStub) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		body, isSearch := s.payload, !strings.Contains(r.URL.Path, "/events/")
		s.calls++
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if isSearch {
			_, _ = fmt.Fprintf(w, `{"_embedded":{"events":[%s]}}`, body)
			return
		}
		_, _ = fmt.Fprint(w, body)
	}))
}

// --- event payload builder ---

// eventSpec describes a Ticketmaster event the way the API would report it.
// Times are pointers so "absent" is distinguishable from "zero".
type eventSpec struct {
	ID          string
	Name        string
	Status      string // onsale | offsale | cancelled | postponed | rescheduled
	EventDate   *time.Time
	PublicStart *time.Time
	PublicEnd   *time.Time
	// SentinelOnsale emits Ticketmaster's 9999-12-31 placeholder instead of a real
	// start date, i.e. "onsale date not announced".
	SentinelOnsale bool
	Presales       []presaleSpec
}

type presaleSpec struct {
	Name  string
	Start time.Time
	End   time.Time
}

func (e eventSpec) json() string {
	iso := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return t.UTC().Format(time.RFC3339)
	}

	sales := map[string]any{}
	pub := map[string]any{}
	switch {
	case e.SentinelOnsale:
		pub["startDateTime"] = "9999-12-31T06:00:00Z"
		// Deliberately false: a live event carrying the sentinel had both flags
		// false, so the parser must not depend on them.
		pub["startTBD"], pub["startTBA"] = false, false
	case e.PublicStart != nil:
		pub["startDateTime"] = iso(e.PublicStart)
	}
	if e.PublicEnd != nil {
		pub["endDateTime"] = iso(e.PublicEnd)
	}
	if len(pub) > 0 {
		sales["public"] = pub
	}
	if len(e.Presales) > 0 {
		var ps []map[string]any
		for _, p := range e.Presales {
			ps = append(ps, map[string]any{
				"name":          p.Name,
				"startDateTime": p.Start.UTC().Format(time.RFC3339),
				"endDateTime":   p.End.UTC().Format(time.RFC3339),
			})
		}
		sales["presales"] = ps
	}

	dates := map[string]any{"status": map[string]any{"code": e.Status}}
	if e.EventDate != nil {
		dates["start"] = map[string]any{"dateTime": iso(e.EventDate)}
	}

	payload := map[string]any{
		"id":    e.ID,
		"name":  e.Name,
		"url":   "https://ticketmaster.test/e/" + e.ID,
		"dates": dates,
		"_embedded": map[string]any{
			"venues": []map[string]any{{"name": "Test Arena"}},
		},
	}
	if len(sales) > 0 {
		payload["sales"] = sales
	}
	b, _ := json.Marshal(payload)
	return string(b)
}

// --- captured email ---

type sentEmail struct {
	To      string
	Subject string
	Text    string
	HTML    string
	Headers map[string]string
}

type capturingSender struct {
	mu   sync.Mutex
	sent []sentEmail
}

func (c *capturingSender) Channel() string { return "email" }
func (c *capturingSender) Send(_ context.Context, m notifier.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sent = append(c.sent, sentEmail{m.To, m.Subject, m.TextBody, m.HTMLBody, m.Headers})
	return nil
}

// --- the harness ---

type harness struct {
	t     *testing.T
	q     *db.Queries
	stub  *tmStub
	tm    *ticketmaster.Client
	svc   *service.WatchService
	mail  *capturingSender
	notif *notifier.Notifier

	// now is the worker's clock. Tests advance it to make time pass without
	// sleeping. It starts at the real current time so it stays consistent with
	// timestamps Postgres writes itself (created_at, last_polled_at).
	now time.Time
}

const harnessSecret = "harness-secret"

func newHarness(t *testing.T) *harness {
	t.Helper()
	q := setupDB(t)

	stub := &tmStub{}
	srv := stub.server()
	t.Cleanup(srv.Close)

	mail := &capturingSender{}
	h := &harness{
		t:     t,
		q:     q,
		stub:  stub,
		tm:    ticketmaster.New(srv.URL, "testkey", nil),
		mail:  mail,
		notif: notifier.New(q, mail),
		now:   time.Now(),
	}
	h.svc = service.NewWatchService(q, h.tm, 0)
	return h
}

// user creates a verified, subscribed account — the state in which alerts are
// allowed to send.
//
// The label is suffixed with a nonce because setupDB deliberately does NOT
// truncate `users` (other tests rely on the migration-seeded demo account), so a
// fixed address would collide the second time the suite runs. Tests that only
// pass on a fresh database are worse than no tests.
func (h *harness) user(label string) db.User {
	h.t.Helper()
	email := fmt.Sprintf("%s+%d@example.test", strings.TrimSuffix(label, "@example.test"), time.Now().UnixNano())
	u, err := h.q.CreateUser(context.Background(), db.CreateUserParams{
		Email: email, PasswordHash: "x",
	})
	if err != nil {
		h.t.Fatalf("create user: %v", err)
	}
	if err := h.q.MarkEmailVerified(context.Background(), u.ID); err != nil {
		h.t.Fatalf("verify user: %v", err)
	}
	u.EmailVerifiedAt.Valid = true
	return u
}

// watch runs the real WatchService.Create against the currently-stubbed event.
//
// It then aligns watches.created_at with the harness clock. Postgres stamps
// created_at with real wall-clock time, which does not move when a test calls
// advance(); without this, a watch created "two days later" in simulated time
// still looks like it was created at the start, and the
// "was this milestone already true when the user subscribed?" check compares
// against the wrong instant.
func (h *harness) watch(u db.User, spec eventSpec) service.WatchView {
	h.t.Helper()
	h.stub.set(spec.json())
	v, err := h.svc.Create(context.Background(), u.ID, service.CreateWatchInput{
		TMEventID: spec.ID, ConditionType: "becomes_available",
	})
	if err != nil {
		h.t.Fatalf("create watch on %s: %v", spec.ID, err)
	}
	h.exec("UPDATE watches SET created_at = $1 WHERE id = $2", h.now, v.ID)
	return v
}

// exec runs test-only SQL (fixture setup the generated queries don't cover).
func (h *harness) exec(sql string, args ...any) {
	h.t.Helper()
	conn, err := pgx.Connect(context.Background(), dbURL())
	if err != nil {
		h.t.Fatalf("harness exec connect: %v", err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(context.Background(), sql, args...); err != nil {
		h.t.Fatalf("harness exec %q: %v", sql, err)
	}
}

// poll runs one real worker cycle against the currently-stubbed event payload.
func (h *harness) poll(spec eventSpec) {
	h.t.Helper()
	h.stub.set(spec.json())

	ev, err := h.q.GetEventByTMID(context.Background(), spec.ID)
	if err != nil {
		h.t.Fatalf("event %s not tracked: %v", spec.ID, err)
	}
	deps := worker.Deps{
		Store:    h.q,
		TM:       h.tm,
		Notifier: h.notif,
		Now:      func() time.Time { return h.now },
		UnsubscribeURL: func(userID int64) string {
			return "https://app.test/api/unsubscribe?token=" + auth.NewUnsubscribeToken(harnessSecret, userID)
		},
	}
	if err := worker.ProcessEvent(context.Background(), ev.ID, deps); err != nil {
		h.t.Fatalf("poll %s: %v", spec.ID, err)
	}
}

// advance moves the harness clock forward.
func (h *harness) advance(d time.Duration) { h.now = h.now.Add(d) }

// emails returns everything sent so far.
func (h *harness) emails() []sentEmail {
	h.mail.mu.Lock()
	defer h.mail.mu.Unlock()
	out := make([]sentEmail, len(h.mail.sent))
	copy(out, h.mail.sent)
	return out
}

// subjects is a compact view for assertions and failure messages.
func (h *harness) subjects() []string {
	var out []string
	for _, e := range h.emails() {
		out = append(out, e.Subject)
	}
	return out
}

// alertKinds reads the kinds actually recorded for a watch, which is the
// database's own record of what the user was told.
func (h *harness) alertKinds(watchID int64) []string {
	h.t.Helper()
	k, err := h.q.ListFiredAlertKinds(context.Background(), watchID)
	if err != nil {
		h.t.Fatalf("list alert kinds: %v", err)
	}
	return k
}

func (h *harness) reloadWatch(watchID, userID int64) db.Watch {
	h.t.Helper()
	w, err := h.q.GetWatch(context.Background(), db.GetWatchParams{ID: watchID, UserID: userID})
	if err != nil {
		h.t.Fatalf("reload watch %d: %v", watchID, err)
	}
	return w
}

func (h *harness) reloadEvent(tmID string) db.Event {
	h.t.Helper()
	e, err := h.q.GetEventByTMID(context.Background(), tmID)
	if err != nil {
		h.t.Fatalf("reload event %s: %v", tmID, err)
	}
	return e
}

// --- assertions ---

// wantNoEmails fails with the actual subjects, which is what you need to debug.
func (h *harness) wantNoEmails(context string) {
	h.t.Helper()
	if got := h.subjects(); len(got) != 0 {
		h.t.Errorf("%s: expected no emails, got %d: %v", context, len(got), got)
	}
}

func (h *harness) wantEmailCount(n int, context string) {
	h.t.Helper()
	if got := h.subjects(); len(got) != n {
		h.t.Errorf("%s: expected %d email(s), got %d: %v", context, n, len(got), got)
	}
}

// wantLastSubjectContains checks the most recent email, since scenarios build up
// a history and the newest one is usually what a step is about.
func (h *harness) wantLastSubjectContains(sub, context string) {
	h.t.Helper()
	all := h.subjects()
	if len(all) == 0 {
		h.t.Fatalf("%s: no emails sent at all", context)
	}
	last := all[len(all)-1]
	if !strings.Contains(strings.ToLower(last), strings.ToLower(sub)) {
		h.t.Errorf("%s: last subject %q does not contain %q", context, last, sub)
	}
}

func hasKind(kinds []string, want string) bool {
	for _, k := range kinds {
		if k == want {
			return true
		}
	}
	return false
}

// --- time helpers, relative to the harness clock ---

func (h *harness) inFuture(d time.Duration) *time.Time { t := h.now.Add(d); return &t }
func (h *harness) inPast(d time.Duration) *time.Time   { t := h.now.Add(-d); return &t }

// --- small helpers used by the edge tests ---

func uniqueEmail(label string) string {
	return fmt.Sprintf("%s+%d@example.test", label, time.Now().UnixNano())
}

func serviceUpdate(status string) service.UpdateWatchInput {
	return service.UpdateWatchInput{Status: &status}
}

func createInput(tmID string) service.CreateWatchInput {
	return service.CreateWatchInput{TMEventID: tmID, ConditionType: "becomes_available"}
}

// newCappedWatchService rebuilds the service with a per-user watch cap, sharing
// the harness's database and stubbed Ticketmaster client.
func newCappedWatchService(h *harness, cap int) *service.WatchService {
	return service.NewWatchService(h.q, h.tm, cap)
}
