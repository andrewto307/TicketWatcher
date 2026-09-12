package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ticket-watcher/internal/auth"
	"ticket-watcher/internal/ratelimit"
	"ticket-watcher/internal/service"
)

const testSecret = "test-secret"

// Services built with nil deps: these tests only exercise handler paths that
// return before any DB / Ticketmaster call (validation, bad input, auth). A nil
// limiter and nil static FS keep the router to just its API surface.
func testRouter() http.Handler {
	return NewRouter(
		service.NewAuthService(nil, testSecret, time.Hour, nil, "http://test"),
		service.NewSearchService(nil),
		service.NewWatchService(nil, nil, 0),
		service.NewNotificationService(nil),
		service.NewAccountService(nil, testSecret),
		testSecret,
		nil,
		nil,
	)
}

func do(t *testing.T, method, target, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, r)
	return rec
}

func validToken(t *testing.T) string {
	t.Helper()
	tok, err := auth.NewToken(testSecret, 1, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestHealthz(t *testing.T) {
	if rec := do(t, http.MethodGet, "/healthz", "", ""); rec.Code != http.StatusOK {
		t.Errorf("healthz = %d, want 200", rec.Code)
	}
}

func TestProtectedRoutes_RequireToken(t *testing.T) {
	if rec := do(t, http.MethodGet, "/api/watches", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no token = %d, want 401", rec.Code)
	}
	if rec := do(t, http.MethodGet, "/api/watches", "", "garbage-token"); rec.Code != http.StatusUnauthorized {
		t.Errorf("bad token = %d, want 401", rec.Code)
	}
}

func TestRegister_WeakPassword_400(t *testing.T) {
	rec := do(t, http.MethodPost, "/api/auth/register", `{"email":"a@b.com","password":"short"}`, "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("weak password = %d, want 400", rec.Code)
	}
}

func TestSearch_MissingQuery_400(t *testing.T) {
	if rec := do(t, http.MethodGet, "/api/search", "", validToken(t)); rec.Code != http.StatusBadRequest {
		t.Errorf("search without q = %d, want 400", rec.Code)
	}
}

// The auth endpoints must shed load from a single client: without this, login is
// open to unlimited password guessing.
func TestAuthEndpoints_RateLimited(t *testing.T) {
	limiter := ratelimit.NewIPLimiter(60, 2) // 2-request burst
	router := NewRouter(
		service.NewAuthService(nil, testSecret, time.Hour, nil, "http://test"),
		service.NewSearchService(nil),
		service.NewWatchService(nil, nil, 0),
		service.NewNotificationService(nil),
		service.NewAccountService(nil, testSecret),
		testSecret,
		limiter,
		nil,
	)

	// A weak password 400s before any DB call, so these exercise the middleware
	// without needing real services behind it.
	post := func() int {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"a@b.com","password":"short"}`))
		r.Header.Set("Fly-Client-IP", "9.9.9.9")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		return rec.Code
	}

	for i := 0; i < 2; i++ {
		if code := post(); code == http.StatusTooManyRequests {
			t.Fatalf("request %d was throttled, want it inside the burst", i+1)
		}
	}
	if code := post(); code != http.StatusTooManyRequests {
		t.Errorf("3rd rapid request = %d, want 429", code)
	}

	// A different client keeps its own budget.
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"a@b.com","password":"short"}`))
	r.Header.Set("Fly-Client-IP", "8.8.8.8")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	if rec.Code == http.StatusTooManyRequests {
		t.Error("a different IP was throttled — the limit must be per client")
	}
}

// Health and authenticated routes must not sit behind the auth throttle.
func TestRateLimit_DoesNotApplyToHealthz(t *testing.T) {
	limiter := ratelimit.NewIPLimiter(60, 1)
	router := NewRouter(
		service.NewAuthService(nil, testSecret, time.Hour, nil, "http://test"),
		service.NewSearchService(nil),
		service.NewWatchService(nil, nil, 0),
		service.NewNotificationService(nil),
		service.NewAccountService(nil, testSecret),
		testSecret,
		limiter,
		nil,
	)
	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("healthz call %d = %d, want 200", i+1, rec.Code)
		}
	}
}

func TestCreateWatch_BadInputs(t *testing.T) {
	token := validToken(t)
	tests := []struct{ name, body string }{
		{"invalid JSON", "not-json"},
		{"invalid condition", `{"tm_event_id":"x","condition_type":"bogus"}`},
		{"price_below no longer accepted", `{"tm_event_id":"x","condition_type":"price_below"}`},
		{"missing event id", `{"condition_type":"becomes_available"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := do(t, http.MethodPost, "/api/watches", tt.body, token); rec.Code != http.StatusBadRequest {
				t.Errorf("%s = %d, want 400", tt.name, rec.Code)
			}
		})
	}
}
