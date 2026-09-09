//go:build integration

package inttest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"ticket-watcher/internal/auth"
	"ticket-watcher/internal/httpapi"
	"ticket-watcher/internal/notifier"
	"ticket-watcher/internal/ratelimit"
	"ticket-watcher/internal/service"
	"ticket-watcher/internal/ticketmaster"
)

// captureMailer stands in for Resend: it records what would have been emailed so
// the test can pull the token out of the link, exactly as a user would by clicking.
type captureMailer struct {
	mu   sync.Mutex
	sent []notifier.Message
}

func (m *captureMailer) Send(_ context.Context, msg notifier.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return nil
}

func (m *captureMailer) last(t *testing.T) notifier.Message {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		t.Fatal("no email was sent")
	}
	return m.sent[len(m.sent)-1]
}

// Signing secret shared by the test server and the helpers that mint tokens the
// way the app would.
const intTestSecret = "int-test-secret"

// unsubscribeTokenFor mints the opt-out token the notifier would embed in an
// alert email for this user.
func unsubscribeTokenFor(userID int64) string {
	return auth.NewUnsubscribeToken(intTestSecret, userID)
}

var tokenRe = regexp.MustCompile(`token=([A-Za-z0-9_\-%]+)`)

// tokenFromEmail pulls the one-time token out of the link in an email body.
func tokenFromEmail(t *testing.T, msg notifier.Message) string {
	t.Helper()
	m := tokenRe.FindStringSubmatch(msg.TextBody)
	if m == nil {
		t.Fatalf("no token= link in email body:\n%s", msg.TextBody)
	}
	tok, err := url.QueryUnescape(m[1])
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// newAuthTestServer wires the real router against the real DB with a capturing
// mailer and an optional inbound limiter.
func newAuthTestServer(t *testing.T, limiter *ratelimit.IPLimiter, maxWatches int) (*httptest.Server, *captureMailer) {
	t.Helper()
	q := setupDB(t)

	tmSrv := fakeTicketmaster()
	t.Cleanup(tmSrv.Close)
	tm := ticketmaster.New(tmSrv.URL, "testkey", nil)

	mail := &captureMailer{}
	secret := intTestSecret
	router := httpapi.NewRouter(
		service.NewAuthService(q, secret, time.Hour, mail, "http://test"),
		service.NewSearchService(tm),
		service.NewWatchService(q, tm, maxWatches),
		service.NewNotificationService(q),
		service.NewAccountService(q, secret),
		secret,
		limiter,
		nil,
	)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv, mail
}

// TestAPI_EmailVerificationAndPasswordReset drives both email-confirmed flows
// end-to-end against a real database: register -> verify -> forgot -> reset ->
// log in with the new password, including the single-use guarantees.
func TestAPI_EmailVerificationAndPasswordReset(t *testing.T) {
	srv, mail := newAuthTestServer(t, nil, 0)

	// Don't chase the verify redirect: the status and Location are the assertion.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	post := func(path, token, body string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	// --- register: account is created unverified, verification email goes out ---
	email := fmt.Sprintf("verify+%d@example.com", time.Now().UnixNano())
	const origPassword = "password123"
	res := post("/api/auth/register", "", fmt.Sprintf(`{"email":%q,"password":%q}`, email, origPassword))
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("register: %d: %s", res.StatusCode, b)
	}
	var reg map[string]string
	_ = json.NewDecoder(res.Body).Decode(&reg)
	res.Body.Close()
	jwt := reg["token"]

	me := func() map[string]any {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/me", nil)
		req.Header.Set("Authorization", "Bearer "+jwt)
		r, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(r.Body).Decode(&out)
		return out
	}

	if v := me()["email_verified"]; v != false {
		t.Fatalf("email_verified = %v right after register, want false", v)
	}
	verifyToken := tokenFromEmail(t, mail.last(t))

	// --- click the emailed link -> verified, and redirected into the app ---
	vres, err := client.Get(srv.URL + "/api/auth/verify?token=" + url.QueryEscape(verifyToken))
	if err != nil {
		t.Fatal(err)
	}
	vres.Body.Close()
	if vres.StatusCode != http.StatusSeeOther {
		t.Errorf("verify = %d, want 303 redirect", vres.StatusCode)
	}
	if loc := vres.Header.Get("Location"); loc != "/?verified=1" {
		t.Errorf("verify Location = %q, want /?verified=1", loc)
	}
	if v := me()["email_verified"]; v != true {
		t.Fatalf("email_verified = %v after verifying, want true", v)
	}

	// --- the same link must not work twice ---
	vres2, err := client.Get(srv.URL + "/api/auth/verify?token=" + url.QueryEscape(verifyToken))
	if err != nil {
		t.Fatal(err)
	}
	vres2.Body.Close()
	if loc := vres2.Header.Get("Location"); loc != "/?verify_error=1" {
		t.Errorf("replayed verify link Location = %q, want /?verify_error=1 (single use)", loc)
	}

	// --- forgot password: 204 regardless, reset link emailed ---
	if r := post("/api/auth/forgot", "", fmt.Sprintf(`{"email":%q}`, email)); r.StatusCode != http.StatusNoContent {
		t.Fatalf("forgot = %d, want 204", r.StatusCode)
	} else {
		r.Body.Close()
	}
	resetToken := tokenFromEmail(t, mail.last(t))
	if resetToken == verifyToken {
		t.Fatal("reset token equals the verification token — purposes must not be interchangeable")
	}

	// An unknown address must look identical, or this endpoint enumerates accounts.
	if r := post("/api/auth/forgot", "", `{"email":"nobody-here@example.com"}`); r.StatusCode != http.StatusNoContent {
		t.Errorf("forgot for unknown email = %d, want 204 (no user enumeration)", r.StatusCode)
	} else {
		r.Body.Close()
	}

	// --- reset the password ---
	const newPassword = "brand-new-password"
	if r := post("/api/auth/reset", "", fmt.Sprintf(`{"token":%q,"password":%q}`, resetToken, newPassword)); r.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("reset = %d: %s", r.StatusCode, b)
	} else {
		r.Body.Close()
	}

	// --- old password is dead, new one works ---
	if r := post("/api/auth/login", "", fmt.Sprintf(`{"email":%q,"password":%q}`, email, origPassword)); r.StatusCode != http.StatusUnauthorized {
		t.Errorf("login with old password = %d, want 401", r.StatusCode)
	} else {
		r.Body.Close()
	}
	if r := post("/api/auth/login", "", fmt.Sprintf(`{"email":%q,"password":%q}`, email, newPassword)); r.StatusCode != http.StatusOK {
		t.Errorf("login with new password = %d, want 200", r.StatusCode)
	} else {
		r.Body.Close()
	}

	// --- the reset link is single-use too ---
	if r := post("/api/auth/reset", "", fmt.Sprintf(`{"token":%q,"password":"yet-another-pass"}`, resetToken)); r.StatusCode != http.StatusBadRequest {
		t.Errorf("replayed reset token = %d, want 400 (single use)", r.StatusCode)
	} else {
		r.Body.Close()
	}
}

// TestAPI_WatchCapEnforced proves one account can't consume unbounded polling
// budget: past the cap, creating a watch is refused.
func TestAPI_WatchCapEnforced(t *testing.T) {
	srv, _ := newAuthTestServer(t, nil, 1) // cap of one watch per user

	email := fmt.Sprintf("cap+%d@example.com", time.Now().UnixNano())
	res, err := http.Post(srv.URL+"/api/auth/register", "application/json",
		strings.NewReader(fmt.Sprintf(`{"email":%q,"password":"password123"}`, email)))
	if err != nil {
		t.Fatal(err)
	}
	var reg map[string]string
	_ = json.NewDecoder(res.Body).Decode(&reg)
	res.Body.Close()
	token := reg["token"]

	createWatch := func() *http.Response {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/watches",
			strings.NewReader(`{"tm_event_id":"TM999","condition_type":"becomes_available"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	if r := createWatch(); r.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("first watch = %d: %s", r.StatusCode, b)
	} else {
		r.Body.Close()
	}

	r := createWatch()
	defer r.Body.Close()
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("watch past the cap = %d, want 403", r.StatusCode)
	}
	var errBody map[string]string
	_ = json.NewDecoder(r.Body).Decode(&errBody)
	if !strings.Contains(errBody["error"], "up to 1") {
		t.Errorf("error message = %q, want it to state the limit", errBody["error"])
	}
}

// TestAPI_AuthRateLimited confirms the inbound throttle is actually wired to the
// public auth routes in the real router.
func TestAPI_AuthRateLimited(t *testing.T) {
	srv, _ := newAuthTestServer(t, ratelimit.NewIPLimiter(60, 3), 0)

	body := `{"email":"nobody@example.com","password":"wrong-password"}`
	var got429 bool
	for i := 0; i < 8; i++ {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Fly-Client-IP", "203.0.113.7")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode == http.StatusTooManyRequests {
			if res.Header.Get("Retry-After") == "" {
				t.Error("429 response has no Retry-After header")
			}
			got429 = true
			break
		}
	}
	if !got429 {
		t.Error("8 rapid login attempts were never throttled — brute force is unbounded")
	}
}
