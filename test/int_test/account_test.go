//go:build integration

package inttest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestAPI_UnsubscribeAndResubscribe drives the opt-out path the way a recipient
// actually would: take the link out of a sent email and open it, with no session.
func TestAPI_UnsubscribeAndResubscribe(t *testing.T) {
	srv, mail := newAuthTestServer(t, nil, 0)

	// The unsubscribe link only reaches a user via email, so register to get one
	// generated, then read it back out of the captured verification message flow.
	email := fmt.Sprintf("unsub+%d@example.com", time.Now().UnixNano())
	res, err := http.Post(srv.URL+"/api/auth/register", "application/json",
		strings.NewReader(fmt.Sprintf(`{"email":%q,"password":"password123"}`, email)))
	if err != nil {
		t.Fatal(err)
	}
	var reg map[string]string
	_ = json.NewDecoder(res.Body).Decode(&reg)
	res.Body.Close()
	token := reg["token"]
	_ = mail.last(t) // verification email was sent

	me := func() map[string]any {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(r.Body).Decode(&out)
		return out
	}

	if v := me()["unsubscribed"]; v != false {
		t.Fatalf("unsubscribed = %v on a new account, want false", v)
	}

	// Build the link the same way the notifier does.
	userID := int64(me()["id"].(float64))
	unsubURL := srv.URL + "/api/unsubscribe?token=" + unsubscribeTokenFor(userID)

	// --- click it, unauthenticated, as an email client would ---
	r, err := http.Get(unsubURL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("unsubscribe = %d, want 200", r.StatusCode)
	}
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want HTML (a human opened this from their inbox)", ct)
	}
	if !strings.Contains(string(body), "unsubscribed") {
		t.Errorf("page doesn't confirm the opt-out: %.200s", body)
	}
	if v := me()["unsubscribed"]; v != true {
		t.Fatalf("unsubscribed = %v after opting out, want true", v)
	}

	// --- clicking twice must not error (idempotent) ---
	r2, err := http.Get(unsubURL)
	if err != nil {
		t.Fatal(err)
	}
	r2.Body.Close()
	if r2.StatusCode != http.StatusOK {
		t.Errorf("second unsubscribe = %d, want 200 (must be idempotent)", r2.StatusCode)
	}

	// --- RFC 8058 one-click POST, what Gmail's own button calls ---
	r3, err := http.Post(unsubURL, "application/x-www-form-urlencoded",
		strings.NewReader("List-Unsubscribe=One-Click"))
	if err != nil {
		t.Fatal(err)
	}
	r3.Body.Close()
	if r3.StatusCode != http.StatusOK {
		t.Errorf("one-click POST = %d, want 200", r3.StatusCode)
	}

	// --- a forged token must be refused ---
	r4, err := http.Get(srv.URL + "/api/unsubscribe?token=999.forged")
	if err != nil {
		t.Fatal(err)
	}
	r4.Body.Close()
	if r4.StatusCode != http.StatusBadRequest {
		t.Errorf("forged token = %d, want 400", r4.StatusCode)
	}

	// --- resubscribe from the UI (authenticated) ---
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/account/resubscribe", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r5, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r5.Body.Close()
	if r5.StatusCode != http.StatusNoContent {
		t.Fatalf("resubscribe = %d, want 204", r5.StatusCode)
	}
	if v := me()["unsubscribed"]; v != false {
		t.Errorf("unsubscribed = %v after resubscribing, want false", v)
	}
}

// TestAPI_DeleteAccount proves erasure actually erases: the account is gone, its
// watches go with it, and the old token stops working.
func TestAPI_DeleteAccount(t *testing.T) {
	srv, _ := newAuthTestServer(t, nil, 0)

	email := fmt.Sprintf("erase+%d@example.com", time.Now().UnixNano())
	res, err := http.Post(srv.URL+"/api/auth/register", "application/json",
		strings.NewReader(fmt.Sprintf(`{"email":%q,"password":"password123"}`, email)))
	if err != nil {
		t.Fatal(err)
	}
	var reg map[string]string
	_ = json.NewDecoder(res.Body).Decode(&reg)
	res.Body.Close()
	token := reg["token"]

	authed := func(method, path string, body string) *http.Response {
		t.Helper()
		var rdr io.Reader
		if body != "" {
			rdr = strings.NewReader(body)
		}
		req, _ := http.NewRequest(method, srv.URL+path, rdr)
		req.Header.Set("Authorization", "Bearer "+token)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	// Give the account something to cascade.
	if r := authed(http.MethodPost, "/api/watches", `{"tm_event_id":"TM999","condition_type":"becomes_available"}`); r.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("create watch: %d: %s", r.StatusCode, b)
	} else {
		r.Body.Close()
	}

	// --- delete ---
	if r := authed(http.MethodDelete, "/api/account", ""); r.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("delete account = %d: %s", r.StatusCode, b)
	} else {
		r.Body.Close()
	}

	// --- the token now names a user that doesn't exist ---
	// 401, not 500: the JWT is still structurally valid and sitting in the
	// browser, so this is an auth failure the client can act on by returning to
	// login. A 500 would surface as "internal error" and strand the user.
	r := authed(http.MethodGet, "/api/me", "")
	r.Body.Close()
	if r.StatusCode != http.StatusUnauthorized {
		t.Errorf("/api/me after deletion = %d, want 401", r.StatusCode)
	}

	// --- and the credentials are gone ---
	lr, err := http.Post(srv.URL+"/api/auth/login", "application/json",
		strings.NewReader(fmt.Sprintf(`{"email":%q,"password":"password123"}`, email)))
	if err != nil {
		t.Fatal(err)
	}
	lr.Body.Close()
	if lr.StatusCode != http.StatusUnauthorized {
		t.Errorf("login after deletion = %d, want 401", lr.StatusCode)
	}
}
