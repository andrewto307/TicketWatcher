//go:build integration

package inttest

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"ticket-watcher/internal/store"
	"ticket-watcher/internal/store/db"
)

func dbURL() string {
	if u := os.Getenv("DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://watcher:watcher@localhost:5432/watcher?sslmode=disable"
}

// setupDB migrates, connects, and truncates for a clean slate. It SKIPs (not
// fails) when Postgres is unavailable, so the suite is safe to run anywhere.
func setupDB(t *testing.T) *db.Queries {
	t.Helper()
	url := dbURL()
	if err := store.Migrate(url); err != nil {
		t.Skipf("Postgres unavailable (%v) — run: docker compose up -d db", err)
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Skipf("Postgres unavailable: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		"TRUNCATE events, watches, availability_snapshots, notifications RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	t.Cleanup(pool.Close)
	return db.New(pool)
}
