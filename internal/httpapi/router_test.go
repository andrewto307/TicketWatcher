package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ticket-watcher/internal/auth"
	"ticket-watcher/internal/service"
)

const testSecret = "test-secret"

// Services built with nil deps: these tests only exercise handler paths that
// return before any DB / Ticketmaster call (validation, bad input, auth).
func testRouter() http.Handler {
	return NewRouter(
		service.NewAuthService(nil, testSecret, time.Hour),
		service.NewSearchService(nil),
		service.NewWatchService(nil, nil),
		service.NewNotificationService(nil),
		testSecret,
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

func TestCreateWatch_BadInputs(t *testing.T) {
	token := validToken(t)
	tests := []struct{ name, body string }{
		{"invalid JSON", "not-json"},
		{"invalid condition", `{"tm_event_id":"x","condition_type":"bogus"}`},
		{"price_below without threshold", `{"tm_event_id":"x","condition_type":"price_below"}`},
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
