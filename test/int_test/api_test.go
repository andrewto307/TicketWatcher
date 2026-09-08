//go:build integration

package inttest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

// TestAPI_AuthAndWatchLifecycle drives the real HTTP router + real DB + real
// client (fake upstream) through register → authed CRUD across every endpoint,
// plus the 401 path.
func TestAPI_AuthAndWatchLifecycle(t *testing.T) {
	q := setupDB(t)

	tmSrv := fakeTicketmaster()
	defer tmSrv.Close()
	tm := ticketmaster.New(tmSrv.URL, "testkey", nil)

	const secret = "int-test-secret"
	router := httpapi.NewRouter(
		service.NewAuthService(q, secret, time.Hour, nil, "http://test"),
		service.NewSearchService(tm),
		service.NewWatchService(q, tm, 0),
		service.NewNotificationService(q),
		secret,
		nil,
		nil,
	)
	api := httptest.NewServer(router)
	defer api.Close()

	// authed request helper
	req := func(method, path, token, body string) *http.Response {
		t.Helper()
		var r io.Reader
		if body != "" {
			r = strings.NewReader(body)
		}
		hr, _ := http.NewRequest(method, api.URL+path, r)
		if body != "" {
			hr.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			hr.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(hr)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	decode := func(res *http.Response) []map[string]any {
		var out []map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		res.Body.Close()
		return out
	}

	// --- protected route without a token -> 401 ---
	if res := req(http.MethodGet, "/api/watches", "", ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", res.StatusCode)
	}

	// --- register -> token ---
	// Unique email per run: setupDB doesn't truncate `users` (the seeded demo
	// user is relied on by other tests), so a fixed email would 409 on re-runs.
	email := fmt.Sprintf("user+%d@example.com", time.Now().UnixNano())
	res := req(http.MethodPost, "/api/auth/register", "", fmt.Sprintf(`{"email":%q,"password":"password123"}`, email))
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("register: %d: %s", res.StatusCode, b)
	}
	var reg map[string]string
	_ = json.NewDecoder(res.Body).Decode(&reg)
	res.Body.Close()
	token := reg["token"]
	if token == "" {
		t.Fatal("register returned no token")
	}

	// --- search ---
	if res, found := req(http.MethodGet, "/api/search?q=fake", token, ""), []map[string]any(nil); true {
		found = decode(res)
		if res.StatusCode != 200 || len(found) != 1 || found[0]["name"] != "Fake Fest" {
			t.Fatalf("search: status=%d results=%+v", res.StatusCode, found)
		}
	}

	// --- create watch ---
	if res := req(http.MethodPost, "/api/watches", token, `{"tm_event_id":"TM999","condition_type":"price_below","threshold":100}`); res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("create: status=%d body=%s", res.StatusCode, b)
	} else {
		res.Body.Close()
	}

	// --- list -> capture id ---
	watches := decode(req(http.MethodGet, "/api/watches", token, ""))
	if len(watches) != 1 || watches[0]["event_name"] != "Fake Fest" {
		t.Fatalf("list watches = %+v", watches)
	}
	id := int64(watches[0]["id"].(float64))

	// --- history (empty) + notifications (empty) ---
	if res, hist := req(http.MethodGet, fmt.Sprintf("/api/watches/%d/history", id), token, ""), []map[string]any(nil); true {
		hist = decode(res)
		if res.StatusCode != 200 || len(hist) != 0 {
			t.Errorf("history: status=%d len=%d, want 200 + empty", res.StatusCode, len(hist))
		}
	}
	if notes := decode(req(http.MethodGet, "/api/notifications", token, "")); len(notes) != 0 {
		t.Errorf("notifications len=%d, want 0", len(notes))
	}

	// --- PATCH threshold ---
	res = req(http.MethodPatch, fmt.Sprintf("/api/watches/%d", id), token, `{"threshold":200}`)
	var updated map[string]any
	_ = json.NewDecoder(res.Body).Decode(&updated)
	res.Body.Close()
	if res.StatusCode != 200 || updated["threshold"].(float64) != 200 {
		t.Errorf("patch: status=%d threshold=%v, want 200 + 200", res.StatusCode, updated["threshold"])
	}

	// --- PATCH invalid status -> 400 ---
	if res := req(http.MethodPatch, fmt.Sprintf("/api/watches/%d", id), token, `{"status":"bogus"}`); res.StatusCode != http.StatusBadRequest {
		t.Errorf("patch invalid status = %d, want 400", res.StatusCode)
	} else {
		res.Body.Close()
	}

	// --- DELETE -> 204, then history 404, then empty list ---
	if res := req(http.MethodDelete, fmt.Sprintf("/api/watches/%d", id), token, ""); res.StatusCode != http.StatusNoContent {
		t.Errorf("delete = %d, want 204", res.StatusCode)
	} else {
		res.Body.Close()
	}
	if res := req(http.MethodGet, fmt.Sprintf("/api/watches/%d/history", id), token, ""); res.StatusCode != http.StatusNotFound {
		t.Errorf("history after delete = %d, want 404", res.StatusCode)
		res.Body.Close()
	}
	if watches := decode(req(http.MethodGet, "/api/watches", token, "")); len(watches) != 0 {
		t.Errorf("after delete, watches = %d, want 0", len(watches))
	}
}
