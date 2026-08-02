package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ticket-watcher/internal/service"
)

// Services built with nil deps: these tests only exercise handler paths that
// return before any DB / Ticketmaster call (validation, bad input, health).
func testRouter() http.Handler {
	return NewRouter(service.NewSearchService(nil), service.NewWatchService(nil, nil, 1))
}

func do(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	testRouter().ServeHTTP(rec, r)
	return rec
}

func TestHealthz(t *testing.T) {
	if rec := do(t, http.MethodGet, "/healthz", ""); rec.Code != http.StatusOK {
		t.Errorf("healthz = %d, want 200", rec.Code)
	}
}

func TestSearch_MissingQuery_400(t *testing.T) {
	if rec := do(t, http.MethodGet, "/api/search", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("search without q = %d, want 400", rec.Code)
	}
}

func TestCreateWatch_BadInputs(t *testing.T) {
	tests := []struct {
		name, body string
	}{
		{"invalid JSON", "not-json"},
		{"invalid condition", `{"tm_event_id":"x","condition_type":"bogus"}`},
		{"price_below without threshold", `{"tm_event_id":"x","condition_type":"price_below"}`},
		{"missing event id", `{"condition_type":"becomes_available"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := do(t, http.MethodPost, "/api/watches", tt.body); rec.Code != http.StatusBadRequest {
				t.Errorf("%s = %d, want 400", tt.name, rec.Code)
			}
		})
	}
}
