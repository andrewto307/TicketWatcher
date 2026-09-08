// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for the service.
type Config struct {
	DatabaseURL string
	TMAPIKey    string // Ticketmaster Discovery API "Consumer Key"
	TMBaseURL   string
	HTTPAddr    string

	// Rate limiting (see plan/05-rate-limiting.md)
	RatePerSec        float64
	RateBurst         int
	DailyBudgetPoll   int
	DailyBudgetSearch int

	// Background engine
	SchedulerInterval time.Duration
	MaxEventsPerTick  int
	WorkerCount       int

	// Notifications
	ResendAPIKey string // if empty, the app uses a log-only email sender
	NotifyFrom   string

	// Auth (Phase 5)
	JWTSecret string
	JWTTTL    time.Duration

	// Tier 1 hardening (see plan/production-readiness/tier-1-safe-for-strangers.md)
	AppBaseURL        string  // public origin, used to build emailed links
	AuthRatePerMin    float64 // per-IP budget on /api/auth/*
	AuthRateBurst     int
	MaxWatchesPerUser int // protects the shared Ticketmaster budget; 0 = unlimited
}

// Load reads configuration from the environment, applying sensible defaults.
// Only TM_API_KEY is strictly required.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:       getenv("DATABASE_URL", "postgres://watcher:watcher@localhost:5432/watcher?sslmode=disable"),
		TMAPIKey:          os.Getenv("TM_API_KEY"),
		TMBaseURL:         getenv("TM_BASE_URL", "https://app.ticketmaster.com/discovery/v2"),
		HTTPAddr:          getenv("HTTP_ADDR", ":8080"),
		RatePerSec:        getenvFloat("TM_RATE_PER_SEC", 5),
		RateBurst:         getenvInt("TM_RATE_BURST", 5),
		DailyBudgetPoll:   getenvInt("TM_DAILY_BUDGET_POLL", 4000),
		DailyBudgetSearch: getenvInt("TM_DAILY_BUDGET_SEARCH", 1000),
		SchedulerInterval: getenvDuration("SCHEDULER_INTERVAL", 15*time.Second),
		MaxEventsPerTick:  getenvInt("MAX_EVENTS_PER_TICK", 20),
		WorkerCount:       getenvInt("WORKER_COUNT", 4),
		ResendAPIKey:      os.Getenv("RESEND_API_KEY"),
		NotifyFrom:        getenv("NOTIFY_FROM", "alerts@example.com"),
		JWTSecret:         getenv("JWT_SECRET", "dev-secret-change-me-in-prod"),
		JWTTTL:            getenvDuration("JWT_TTL", 24*time.Hour),
		AppBaseURL:        strings.TrimRight(getenv("APP_BASE_URL", "http://localhost:8080"), "/"),
		AuthRatePerMin:    getenvFloat("AUTH_RATE_PER_MIN", 10),
		AuthRateBurst:     getenvInt("AUTH_RATE_BURST", 5),
		MaxWatchesPerUser: getenvInt("MAX_WATCHES_PER_USER", 50),
	}
	if cfg.TMAPIKey == "" {
		return Config{}, errors.New("TM_API_KEY is required (get your Consumer Key at https://developer.ticketmaster.com/my-apps)")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getenvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
