// Package inttest holds opt-in integration tests that exercise the app through
// its public surface against real infrastructure (Postgres, the HTTP router,
// the Ticketmaster client over HTTP).
//
// They are guarded by the `integration` build tag so the default `go test ./...`
// stays fast and hermetic. Run them explicitly (Postgres must be up):
//
//	docker compose up -d db
//	go test -tags=integration ./test/...
//
// The DATABASE_URL env var overrides the default localhost connection. Each test
// TRUNCATEs the mutable tables for isolation, so running these WILL clear local
// dev data.
package inttest
