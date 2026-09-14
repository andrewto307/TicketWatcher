package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The senders are the last hop before a real person's inbox, and they run only
// in production — a mistake here is invisible locally and costs a missed alert.
// These tests drive the real Send method against a real HTTP server so the wire
// format, not a mock of it, is what gets checked.

// captured is one request as the Resend API would have received it.
type captured struct {
	method string
	path   string
	auth   string
	ctype  string
	body   map[string]any
}

// resendStub stands in for api.resend.com and records what arrived.
func resendStub(t *testing.T, status int, respBody string) (*ResendSender, *[]captured) {
	t.Helper()
	var got []captured
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Errorf("request body is not valid JSON: %v (%s)", err, raw)
		}
		got = append(got, captured{
			method: r.Method,
			path:   r.URL.Path,
			auth:   r.Header.Get("Authorization"),
			ctype:  r.Header.Get("Content-Type"),
			body:   parsed,
		})
		w.WriteHeader(status)
		_, _ = io.WriteString(w, respBody)
	}))
	t.Cleanup(srv.Close)

	s := NewResendSender("re_test_key", "TicketWatcher <alerts@example.org>")
	s.endpoint = srv.URL + "/emails"
	return s, &got
}

func mustSend(t *testing.T, s *ResendSender, msg Message) {
	t.Helper()
	if err := s.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestResendSender_SendsTheDocumentedPayload(t *testing.T) {
	s, got := resendStub(t, 200, `{"id":"abc"}`)

	mustSend(t, s, Message{
		To:       "user@example.test",
		Subject:  "Tickets are on sale",
		HTMLBody: "<p>go</p>",
		TextBody: "go",
	})

	if len(*got) != 1 {
		t.Fatalf("got %d requests, want 1", len(*got))
	}
	req := (*got)[0]

	if req.method != http.MethodPost {
		t.Errorf("method = %s, want POST", req.method)
	}
	if req.path != "/emails" {
		t.Errorf("path = %s, want /emails", req.path)
	}
	if req.auth != "Bearer re_test_key" {
		t.Errorf("Authorization = %q, want the bearer key", req.auth)
	}
	if req.ctype != "application/json" {
		t.Errorf("Content-Type = %q; Resend rejects a missing JSON content type", req.ctype)
	}

	// from comes from configuration, not from the message: a caller must not be
	// able to spoof the sending identity, and only the configured address is
	// DKIM-signed for the domain.
	if got, want := req.body["from"], "TicketWatcher <alerts@example.org>"; got != want {
		t.Errorf("from = %v, want %v", got, want)
	}
	// "to" must be an array — Resend rejects a bare string.
	to, ok := req.body["to"].([]any)
	if !ok || len(to) != 1 || to[0] != "user@example.test" {
		t.Errorf("to = %#v, want [\"user@example.test\"]", req.body["to"])
	}
	if got, want := req.body["subject"], "Tickets are on sale"; got != want {
		t.Errorf("subject = %v, want %v", got, want)
	}
	if got, want := req.body["html"], "<p>go</p>"; got != want {
		t.Errorf("html = %v, want %v", got, want)
	}
	// Both parts are always sent: the text alternative is what keeps the message
	// out of spam filters and readable in plain-text clients.
	if got, want := req.body["text"], "go"; got != want {
		t.Errorf("text = %v, want %v", got, want)
	}
}

func TestResendSender_HeadersOnlyWhenPresent(t *testing.T) {
	t.Run("omitted for transactional mail", func(t *testing.T) {
		s, got := resendStub(t, 200, `{}`)

		mustSend(t, s, Message{To: "u@example.test", Subject: "Verify your email", TextBody: "link"})

		// An empty headers object is rejected by some Resend API versions, and
		// transactional mail must not advertise an unsubscribe link at all.
		if _, present := (*got)[0].body["headers"]; present {
			t.Errorf("headers sent for a message with none: %#v", (*got)[0].body["headers"])
		}
	})

	t.Run("forwarded verbatim for alerts", func(t *testing.T) {
		s, got := resendStub(t, 200, `{}`)

		mustSend(t, s, Message{
			To:      "u@example.test",
			Subject: "On sale now",
			Headers: map[string]string{
				"List-Unsubscribe":      "<https://app.example.org/api/unsubscribe?t=tok>",
				"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
			},
		})

		h, ok := (*got)[0].body["headers"].(map[string]any)
		if !ok {
			t.Fatalf("headers = %#v, want an object", (*got)[0].body["headers"])
		}
		if h["List-Unsubscribe"] != "<https://app.example.org/api/unsubscribe?t=tok>" {
			t.Errorf("List-Unsubscribe = %v", h["List-Unsubscribe"])
		}
		// Gmail's one-click requirement: without the -Post header the button is
		// not shown, and bulk mail starts landing in spam.
		if h["List-Unsubscribe-Post"] != "List-Unsubscribe=One-Click" {
			t.Errorf("List-Unsubscribe-Post = %v", h["List-Unsubscribe-Post"])
		}
	})
}

func TestResendSender_ReportsAPIFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		// wantReason is a fragment of the provider's own explanation that must
		// reach the log — it is the only diagnostic for a silent non-delivery.
		// This is the 422 we actually hit in production when the alerts.toandrew.com
		// domain was not yet verified.
		wantReason string
	}{
		{"unverified domain", 422, `{"message":"The alerts@example.org domain is not verified"}`, "domain is not verified"},
		{"bad key", 401, `{"message":"API key is invalid"}`, "API key is invalid"},
		{"rate limited", 429, `{"message":"Too many requests"}`, "Too many requests"},
		{"server error", 500, `{"message":"internal"}`, "internal"},
		// 3xx is not a success: a redirect means the mail was never accepted.
		{"redirect", 301, ``, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := resendStub(t, tt.status, tt.body)

			err := s.Send(context.Background(), Message{To: "u@example.test"})

			if err == nil {
				t.Fatalf("status %d reported as success; the alert would be recorded as sent and never retried", tt.status)
			}
			if !strings.Contains(err.Error(), strconv.Itoa(tt.status)) {
				t.Errorf("error %q does not mention status %d", err, tt.status)
			}
			if tt.wantReason != "" && !strings.Contains(err.Error(), tt.wantReason) {
				t.Errorf("error %q drops the provider's explanation %q", err, tt.wantReason)
			}
		})
	}
}

func TestResendSender_Accepts2xx(t *testing.T) {
	for _, status := range []int{200, 201, 202} {
		s, _ := resendStub(t, status, `{"id":"x"}`)
		if err := s.Send(context.Background(), Message{To: "u@example.test"}); err != nil {
			t.Errorf("status %d treated as failure: %v", status, err)
		}
	}
}

// A hostile or broken provider could stream an unbounded error body; the reader
// is limited so one bad response cannot exhaust memory.
func TestResendSender_TruncatesHugeErrorBodies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write(bytes.Repeat([]byte("x"), 1<<20))
	}))
	defer srv.Close()
	s := NewResendSender("k", "f@example.org")
	s.endpoint = srv.URL

	err := s.Send(context.Background(), Message{To: "u@example.test"})

	if err == nil {
		t.Fatal("want an error")
	}
	if len(err.Error()) > 2000 {
		t.Errorf("error is %d bytes; the response body is not being limited", len(err.Error()))
	}
}

func TestResendSender_PropagatesTransportFailures(t *testing.T) {
	s := NewResendSender("k", "f@example.org")
	// A port nothing listens on: the connection is refused immediately.
	s.endpoint = "http://127.0.0.1:1/emails"

	if err := s.Send(context.Background(), Message{To: "u@example.test"}); err == nil {
		t.Fatal("an unreachable provider was reported as a successful send")
	}
}

func TestResendSender_ReportsAnUnbuildableRequest(t *testing.T) {
	s := NewResendSender("k", "f@example.org")
	s.endpoint = "http://exa\x7fmple.com/emails" // control character: rejected by url.Parse

	if err := s.Send(context.Background(), Message{To: "u@example.test"}); err == nil {
		t.Fatal("a request that could not even be built was reported as sent")
	}
}

func TestResendSender_HonoursContextCancellation(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(200)
	}))
	defer srv.Close()
	defer close(release)

	s := NewResendSender("k", "f@example.org")
	s.endpoint = srv.URL

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := s.Send(ctx, Message{To: "u@example.test"})

	if err == nil {
		t.Fatal("want an error when the context expires mid-request")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want a deadline-exceeded cause", err)
	}
	// Shutdown must not wait on the 10s client timeout.
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Send blocked for %v after the context expired", elapsed)
	}
}

func TestResendSender_ChannelIsEmail(t *testing.T) {
	// The channel string is persisted on every notification row and is what the
	// UI filters on; changing it orphans existing history.
	if got := NewResendSender("k", "f@example.org").Channel(); got != "email" {
		t.Errorf("Channel() = %q, want %q", got, "email")
	}
}

func TestResendSender_UsesTheRealEndpointByDefault(t *testing.T) {
	// Nothing else asserts this, and a typo would fail only in production.
	if got := NewResendSender("k", "f@example.org").endpoint; got != "https://api.resend.com/emails" {
		t.Errorf("default endpoint = %q", got)
	}
}

func TestResendSender_HasARequestTimeout(t *testing.T) {
	// Without one, a hung provider would pin a worker goroutine forever and the
	// single-instance scheduler would stop making progress.
	s := NewResendSender("k", "f@example.org")
	if s.http.Timeout == 0 {
		t.Error("the HTTP client has no timeout")
	}
}

// --- LogSender: the default in local dev and the only place verification and
// password-reset links appear when no provider is configured. ---

func TestLogSender_PrintsTheBodySoLinksAreUsable(t *testing.T) {
	var buf bytes.Buffer
	restore := captureLog(&buf)
	defer restore()

	err := (&LogSender{}).Send(context.Background(), Message{
		To:       "user@example.test",
		Subject:  "Confirm your email",
		TextBody: "Open this link:\nhttps://app.example.org/api/auth/verify?token=tok-123",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "user@example.test") {
		t.Error("recipient missing from the log line")
	}
	if !strings.Contains(out, "Confirm your email") {
		t.Error("subject missing from the log line")
	}
	// This is the point of the sender: with no provider configured, a link that
	// is not logged cannot be followed, and local signup is impossible.
	if !strings.Contains(out, "https://app.example.org/api/auth/verify?token=tok-123") {
		t.Errorf("the verification link was not logged:\n%s", out)
	}
	if !strings.Contains(out, "[notify:email]") {
		t.Error("missing the [notify:email] prefix the deploy notes tell you to grep for")
	}
}

func TestLogSender_IndentsMultiLineBodies(t *testing.T) {
	var buf bytes.Buffer
	restore := captureLog(&buf)
	defer restore()

	_ = (&LogSender{}).Send(context.Background(), Message{To: "u@example.test", TextBody: "one\ntwo"})

	out := buf.String()
	if !strings.Contains(out, "    one\n    two") {
		t.Errorf("continuation lines are not indented, so the body is indistinguishable from other log output:\n%s", out)
	}
}

func TestLogSender_EmptyBodyDoesNotAddAStrayIndent(t *testing.T) {
	var buf bytes.Buffer
	restore := captureLog(&buf)
	defer restore()

	_ = (&LogSender{}).Send(context.Background(), Message{To: "u@example.test", Subject: "s"})

	if strings.Contains(buf.String(), "    \n") {
		t.Error("an empty body produced a blank indented line")
	}
}

func TestLogSender_ChannelIsEmail(t *testing.T) {
	if got := NewLogSender().Channel(); got != "email" {
		t.Errorf("Channel() = %q, want %q", got, "email")
	}
}

// Both senders must satisfy Sender, or main.go's fallback would not compile.
var (
	_ Sender = (*ResendSender)(nil)
	_ Sender = (*LogSender)(nil)
)

func captureLog(buf *bytes.Buffer) func() {
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(buf)
	log.SetFlags(0)
	return func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	}
}
