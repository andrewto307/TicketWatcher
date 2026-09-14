package config

import (
	"strings"
	"testing"
	"time"
)

// Configuration is the app's largest silent-failure surface: nothing here errors
// when it is wrong, it just behaves differently. A mistyped variable name or a
// bad default produces a running, healthy-looking app that does the wrong thing —
// e.g. an APP_BASE_URL stuck on localhost puts a dead link in every verification,
// reset and unsubscribe email, with no log line to show for it.
//
// These tests therefore pin the exact env var NAMES and the exact defaults.

// Every variable Load consults. Tests clear all of them so a value exported in
// the developer's own shell (a real DATABASE_URL, say) cannot make a default
// test pass or fail for the wrong reason.
var allVars = []string{
	"DATABASE_URL", "TM_API_KEY", "TM_BASE_URL", "HTTP_ADDR",
	"TM_RATE_PER_SEC", "TM_RATE_BURST", "TM_DAILY_BUDGET_POLL", "TM_DAILY_BUDGET_SEARCH",
	"SCHEDULER_INTERVAL", "MAX_EVENTS_PER_TICK", "WORKER_COUNT",
	"RESEND_API_KEY", "NOTIFY_FROM",
	"JWT_SECRET", "JWT_TTL",
	"APP_BASE_URL", "AUTH_RATE_PER_MIN", "AUTH_RATE_BURST", "MAX_WATCHES_PER_USER",
}

// clearEnv unsets everything for the duration of one test; t.Setenv restores the
// previous values (and fails the test if it is run in parallel).
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range allVars {
		t.Setenv(k, "")
	}
}

// withEnv loads a Config from a clean environment plus the given overrides.
// TM_API_KEY is always present because Load fails without it.
func withEnv(t *testing.T, kv map[string]string) Config {
	t.Helper()
	clearEnv(t)
	t.Setenv("TM_API_KEY", "test-key")
	for k, v := range kv {
		t.Setenv(k, v)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestLoad_RequiresTicketmasterKey(t *testing.T) {
	clearEnv(t)

	cfg, err := Load()

	if err == nil {
		t.Fatal("Load succeeded without TM_API_KEY; the app would start and every poll would 401")
	}
	// A partially-populated Config must not escape alongside the error — a caller
	// that ignores err would otherwise run with a silently keyless client.
	if cfg != (Config{}) {
		t.Errorf("Load returned a populated Config with its error: %+v", cfg)
	}
	// The message should tell the reader where to get a key.
	if got := err.Error(); !strings.Contains(got, "TM_API_KEY") {
		t.Errorf("unhelpful error: %q", got)
	}
}

func TestLoad_Defaults(t *testing.T) {
	cfg := withEnv(t, nil)

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"DatabaseURL", cfg.DatabaseURL, "postgres://watcher:watcher@localhost:5432/watcher?sslmode=disable"},
		{"TMBaseURL", cfg.TMBaseURL, "https://app.ticketmaster.com/discovery/v2"},
		{"HTTPAddr", cfg.HTTPAddr, ":8080"},
		{"RatePerSec", cfg.RatePerSec, 5.0},
		{"RateBurst", cfg.RateBurst, 5},
		{"DailyBudgetPoll", cfg.DailyBudgetPoll, 4000},
		{"DailyBudgetSearch", cfg.DailyBudgetSearch, 1000},
		{"SchedulerInterval", cfg.SchedulerInterval, 15 * time.Second},
		{"MaxEventsPerTick", cfg.MaxEventsPerTick, 20},
		{"WorkerCount", cfg.WorkerCount, 4},
		{"NotifyFrom", cfg.NotifyFrom, "alerts@example.com"},
		{"JWTTTL", cfg.JWTTTL, 24 * time.Hour},
		{"AppBaseURL", cfg.AppBaseURL, "http://localhost:8080"},
		{"AuthRatePerMin", cfg.AuthRatePerMin, 10.0},
		{"AuthRateBurst", cfg.AuthRateBurst, 5},
		{"MaxWatchesPerUser", cfg.MaxWatchesPerUser, 50},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s default = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

// The rate-limit defaults must not exceed Ticketmaster's documented free-tier
// budget, or the app is designed to be throttled.
func TestLoad_DefaultsRespectTheTicketmasterBudget(t *testing.T) {
	cfg := withEnv(t, nil)

	if total := cfg.DailyBudgetPoll + cfg.DailyBudgetSearch; total > 5000 {
		t.Errorf("poll+search budget = %d, exceeds the documented 5,000/day limit", total)
	}
	if cfg.RatePerSec > 5 {
		t.Errorf("RatePerSec = %v, exceeds the documented 5 req/s limit", cfg.RatePerSec)
	}
	// Reserving headroom for interactive search is deliberate (plan/06 D10): a busy
	// poll loop must not starve the user-facing search.
	if cfg.DailyBudgetSearch == 0 {
		t.Error("no budget reserved for interactive search; a busy poll loop would starve it")
	}
}

// Secrets must have no usable default. A silent fallback is how a dev secret
// reaches production.
func TestLoad_SecretsAreNotSilentlyDefaulted(t *testing.T) {
	cfg := withEnv(t, nil)

	if cfg.ResendAPIKey != "" {
		t.Errorf("RESEND_API_KEY defaulted to %q; it must be empty so the app falls back to the log sender", cfg.ResendAPIKey)
	}
	// JWT_SECRET does have a dev default, which is a deliberate convenience — but
	// it must be obviously unusable in production if anyone ever sees it in a log.
	if cfg.JWTSecret == "" {
		t.Error("JWT_SECRET has no default; local dev would fail to sign tokens")
	}
	if !strings.Contains(cfg.JWTSecret, "change-me") {
		t.Errorf("the JWT_SECRET default %q does not announce itself as a dev value", cfg.JWTSecret)
	}
}

// Each variable is read from the exact name documented in the deployment notes.
// A rename here breaks production silently, because the old value is simply ignored.
func TestLoad_ReadsEachDocumentedVariableName(t *testing.T) {
	cfg := withEnv(t, map[string]string{
		"DATABASE_URL":           "postgres://u:p@db:5432/x",
		"TM_BASE_URL":            "https://tm.test/v2",
		"HTTP_ADDR":              ":9999",
		"TM_RATE_PER_SEC":        "2.5",
		"TM_RATE_BURST":          "3",
		"TM_DAILY_BUDGET_POLL":   "100",
		"TM_DAILY_BUDGET_SEARCH": "50",
		"SCHEDULER_INTERVAL":     "42s",
		"MAX_EVENTS_PER_TICK":    "7",
		"WORKER_COUNT":           "11",
		"RESEND_API_KEY":         "re_live",
		"NOTIFY_FROM":            "alerts@example.org",
		"JWT_SECRET":             "a-real-secret",
		"JWT_TTL":                "2h",
		"APP_BASE_URL":           "https://app.example.org",
		"AUTH_RATE_PER_MIN":      "60",
		"AUTH_RATE_BURST":        "9",
		"MAX_WATCHES_PER_USER":   "3",
	})

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"DATABASE_URL", cfg.DatabaseURL, "postgres://u:p@db:5432/x"},
		{"TM_BASE_URL", cfg.TMBaseURL, "https://tm.test/v2"},
		{"HTTP_ADDR", cfg.HTTPAddr, ":9999"},
		{"TM_RATE_PER_SEC", cfg.RatePerSec, 2.5},
		{"TM_RATE_BURST", cfg.RateBurst, 3},
		{"TM_DAILY_BUDGET_POLL", cfg.DailyBudgetPoll, 100},
		{"TM_DAILY_BUDGET_SEARCH", cfg.DailyBudgetSearch, 50},
		{"SCHEDULER_INTERVAL", cfg.SchedulerInterval, 42 * time.Second},
		{"MAX_EVENTS_PER_TICK", cfg.MaxEventsPerTick, 7},
		{"WORKER_COUNT", cfg.WorkerCount, 11},
		{"RESEND_API_KEY", cfg.ResendAPIKey, "re_live"},
		{"NOTIFY_FROM", cfg.NotifyFrom, "alerts@example.org"},
		{"JWT_SECRET", cfg.JWTSecret, "a-real-secret"},
		{"JWT_TTL", cfg.JWTTTL, 2 * time.Hour},
		{"APP_BASE_URL", cfg.AppBaseURL, "https://app.example.org"},
		{"AUTH_RATE_PER_MIN", cfg.AuthRatePerMin, 60.0},
		{"AUTH_RATE_BURST", cfg.AuthRateBurst, 9},
		{"MAX_WATCHES_PER_USER", cfg.MaxWatchesPerUser, 3},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v — is the variable name still spelled correctly?", c.name, c.got, c.want)
		}
	}
}

// APP_BASE_URL is concatenated with paths to build emailed links, so a trailing
// slash would produce "https://app//api/auth/verify".
func TestLoad_AppBaseURLTrailingSlashIsStripped(t *testing.T) {
	for _, in := range []string{"https://app.example.org/", "https://app.example.org///"} {
		cfg := withEnv(t, map[string]string{"APP_BASE_URL": in})
		if got, want := cfg.AppBaseURL, "https://app.example.org"; got != want {
			t.Errorf("APP_BASE_URL %q -> %q, want %q", in, got, want)
		}
	}
}

// An unparseable value falls back to the default rather than crashing or
// silently becoming zero — zero would be catastrophic for a budget or interval.
func TestLoad_MalformedValuesFallBackToDefaults(t *testing.T) {
	cfg := withEnv(t, map[string]string{
		"TM_RATE_PER_SEC":      "not-a-number",
		"WORKER_COUNT":         "abc",
		"SCHEDULER_INTERVAL":   "soon",
		"MAX_WATCHES_PER_USER": "",
	})

	if cfg.RatePerSec != 5 {
		t.Errorf("RatePerSec = %v, want the 5 default", cfg.RatePerSec)
	}
	if cfg.WorkerCount != 4 {
		t.Errorf("WorkerCount = %d, want the 4 default (0 would mean no workers at all)", cfg.WorkerCount)
	}
	if cfg.SchedulerInterval != 15*time.Second {
		t.Errorf("SchedulerInterval = %v, want 15s (0 would spin the ticker)", cfg.SchedulerInterval)
	}
	if cfg.MaxWatchesPerUser != 50 {
		t.Errorf("MaxWatchesPerUser = %d, want the 50 default", cfg.MaxWatchesPerUser)
	}
}

// Values that would break the app if taken literally.
func TestLoad_ZeroValuesAreAcceptedWhenExplicit(t *testing.T) {
	// 0 watches per user is documented as "unlimited", so it must survive.
	cfg := withEnv(t, map[string]string{"MAX_WATCHES_PER_USER": "0"})
	if cfg.MaxWatchesPerUser != 0 {
		t.Errorf("MaxWatchesPerUser = %d, want 0 (documented as unlimited)", cfg.MaxWatchesPerUser)
	}
}
