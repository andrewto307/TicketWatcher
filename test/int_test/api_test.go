//go:build integration

package inttest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ticket-watcher/internal/httpapi"
	"ticket-watcher/internal/service"
	"ticket-watcher/internal/ticketmaster"
)

// fakeTicketmaster stands in for the Discovery API so the API test needs no
// network / real key. The real ticketmaster.Client points at this server.
func fakeTicketmaster() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/events/") { // single-event endpoint
			_, _ = io.WriteString(w, `{"id":"TM999","name":"Fake Fest","url":"http://tm/x","dates":{"start":{"dateTime":"2026-09-01T20:00:00Z"},"status":{"code":"onsale"}},"priceRanges":[{"min":50,"max":150}],"_embedded":{"venues":[{"name":"Dome"}]}}`)
			return
		}
		_, _ = io.WriteString(w, `{"_embedded":{"events":[{"id":"TM999","name":"Fake Fest","dates":{"status":{"code":"onsale"}},"priceRanges":[{"min":50,"max":150}],"_embedded":{"venues":[{"name":"Dome"}]}}]}}`)
	}))
}

// TestAPI_SearchCreateList drives the real HTTP router + real DB + real client
// (against the fake upstream): search -> create watch -> list, plus a 400 case.
func TestAPI_SearchCreateList(t *testing.T) {
	q := setupDB(t)
	user, err := q.GetUserByEmail(context.Background(), "demo@example.com")
	if err != nil {
		t.Fatalf("user: %v", err)
	}

	tmSrv := fakeTicketmaster()
	defer tmSrv.Close()
	tm := ticketmaster.New(tmSrv.URL, "testkey", nil)

	router := httpapi.NewRouter(service.NewSearchService(tm), service.NewWatchService(q, tm, user.ID))
	api := httptest.NewServer(router)
	defer api.Close()

	// --- search ---
	res, err := http.Get(api.URL + "/api/search?q=fake")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("search status = %d", res.StatusCode)
	}
	var found []map[string]any
	_ = json.NewDecoder(res.Body).Decode(&found)
	res.Body.Close()
	if len(found) != 1 || found[0]["name"] != "Fake Fest" {
		t.Fatalf("search results = %+v", found)
	}

	// --- create watch ---
	res, err = http.Post(api.URL+"/api/watches", "application/json",
		strings.NewReader(`{"tm_event_id":"TM999","condition_type":"price_below","threshold":100}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("create status = %d: %s", res.StatusCode, body)
	}
	res.Body.Close()

	// --- list watches (reads it back from Postgres, event info joined in) ---
	res, err = http.Get(api.URL + "/api/watches")
	if err != nil {
		t.Fatal(err)
	}
	var watches []map[string]any
	_ = json.NewDecoder(res.Body).Decode(&watches)
	res.Body.Close()
	if len(watches) != 1 || watches[0]["event_name"] != "Fake Fest" {
		t.Fatalf("watches = %+v", watches)
	}

	// --- invalid create -> 400 ---
	res, err = http.Post(api.URL+"/api/watches", "application/json",
		strings.NewReader(`{"tm_event_id":"TM999","condition_type":"price_below"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("missing-threshold create = %d, want 400", res.StatusCode)
	}
	res.Body.Close()
}
