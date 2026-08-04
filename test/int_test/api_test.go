//go:build integration

package inttest

import (
	"context"
	"encoding/json"
	"fmt"
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

// TestAPI_WatchLifecycle drives the real HTTP router + real DB + real client
// (against the fake upstream) across every Phase 1–4 endpoint.
func TestAPI_WatchLifecycle(t *testing.T) {
	q := setupDB(t)
	user, err := q.GetUserByEmail(context.Background(), "demo@example.com")
	if err != nil {
		t.Fatalf("user: %v", err)
	}

	tmSrv := fakeTicketmaster()
	defer tmSrv.Close()
	tm := ticketmaster.New(tmSrv.URL, "testkey", nil)

	router := httpapi.NewRouter(
		service.NewSearchService(tm),
		service.NewWatchService(q, tm, user.ID),
		service.NewNotificationService(q, user.ID),
	)
	api := httptest.NewServer(router)
	defer api.Close()

	get := func(path string) (*http.Response, []map[string]any) {
		t.Helper()
		res, err := http.Get(api.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		var out []map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		res.Body.Close()
		return res, out
	}

	// --- search ---
	if res, found := get("/api/search?q=fake"); res.StatusCode != 200 || len(found) != 1 || found[0]["name"] != "Fake Fest" {
		t.Fatalf("search: status=%d results=%+v", res.StatusCode, found)
	}

	// --- create watch ---
	res, err := http.Post(api.URL+"/api/watches", "application/json",
		strings.NewReader(`{"tm_event_id":"TM999","condition_type":"price_below","threshold":100}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("create: status=%d body=%s", res.StatusCode, b)
	}
	res.Body.Close()

	// --- list -> capture id ---
	res, watches := get("/api/watches")
	if res.StatusCode != 200 || len(watches) != 1 || watches[0]["event_name"] != "Fake Fest" {
		t.Fatalf("list: status=%d watches=%+v", res.StatusCode, watches)
	}
	id := int64(watches[0]["id"].(float64))

	// --- history (empty; no poller in this test) ---
	if res, hist := get(fmt.Sprintf("/api/watches/%d/history", id)); res.StatusCode != 200 || len(hist) != 0 {
		t.Errorf("history: status=%d, want 200 + empty", res.StatusCode)
	}

	// --- notifications (empty) ---
	if res, notes := get("/api/notifications"); res.StatusCode != 200 || len(notes) != 0 {
		t.Errorf("notifications: status=%d len=%d, want 200 + empty", res.StatusCode, len(notes))
	}

	// --- PATCH threshold ---
	req, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/watches/%d", api.URL, id),
		strings.NewReader(`{"threshold":200}`))
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var updated map[string]any
	_ = json.NewDecoder(res.Body).Decode(&updated)
	res.Body.Close()
	if res.StatusCode != 200 || updated["threshold"].(float64) != 200 {
		t.Errorf("patch: status=%d threshold=%v, want 200 + 200", res.StatusCode, updated["threshold"])
	}

	// --- PATCH invalid status -> 400 ---
	req, _ = http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/watches/%d", api.URL, id),
		strings.NewReader(`{"status":"bogus"}`))
	res, _ = http.DefaultClient.Do(req)
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("patch invalid status = %d, want 400", res.StatusCode)
	}
	res.Body.Close()

	// --- DELETE ---
	req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/watches/%d", api.URL, id), nil)
	res, _ = http.DefaultClient.Do(req)
	if res.StatusCode != http.StatusNoContent {
		t.Errorf("delete = %d, want 204", res.StatusCode)
	}
	res.Body.Close()

	// --- history on a deleted watch -> 404 ---
	if res, _ := get(fmt.Sprintf("/api/watches/%d/history", id)); res.StatusCode != http.StatusNotFound {
		t.Errorf("history after delete = %d, want 404", res.StatusCode)
	}

	// --- list is now empty ---
	if _, watches := get("/api/watches"); len(watches) != 0 {
		t.Errorf("after delete, watches = %d, want 0", len(watches))
	}
}
