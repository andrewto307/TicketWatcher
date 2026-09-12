package ticketmaster

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestClient_Search_OverHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("apikey") != "k" {
			t.Errorf("apikey not sent as query param; got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"_embedded":{"events":[
			{"id":"E1","name":"Show","dates":{"status":{"code":"onsale"}},"priceRanges":[{"min":10,"max":20}]}
		]}}`))
	}))
	defer srv.Close()

	evs, err := New(srv.URL, "k", nil).Search(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Name != "Show" || evs[0].Availability != "onsale" {
		t.Errorf("bad parse: %+v", evs)
	}
}

func TestClient_GetEvent_OverHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"E1","name":"Show","dates":{"status":{"code":"onsale"}}}`))
	}))
	defer srv.Close()

	e, err := New(srv.URL, "k", nil).GetEvent(context.Background(), "E1")
	if err != nil {
		t.Fatal(err)
	}
	if e.TMEventID != "E1" || e.Availability != "onsale" {
		t.Errorf("bad: %+v", e)
	}
}

// Covers the 429 backoff + Retry-After path in do().
func TestClient_Retries429ThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "0") // retry immediately, keep the test fast
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"_embedded":{"events":[]}}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "k", nil).Search(context.Background(), "x"); err != nil {
		t.Fatalf("should succeed after one 429 retry: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("upstream calls = %d, want 2 (429 then 200)", got)
	}
}

func TestClient_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "k", nil).Search(context.Background(), "x"); err == nil {
		t.Error("expected an error on HTTP 500")
	}
}
