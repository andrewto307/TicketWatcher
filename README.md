# TicketWatcher

Track Ticketmaster events and get an email the moment tickets become buyable.

Live at **https://ticketwatcher.fly.dev**

Ticketmaster doesn't notify you when an event goes on sale — you either set a
calendar reminder or keep refreshing the page. TicketWatcher polls the events you
care about and emails you when something actually changes: the onsale date gets
announced, a presale opens, the public sale starts, or the event is cancelled or
rescheduled.

---

## How it works

Search for an event, click watch, and a background loop takes over.

```
                  ┌───────────┐        ┌──────────┐
   React SPA ───► │  Go API   │ ─────► │ Postgres │
                  └───────────┘        └──────────┘
                                            ▲
                        ┌───────────────────┴───────────────────┐
                        │                                       │
                  ┌───────────┐   job queue   ┌──────────────┐  │
                  │ Scheduler │ ────────────► │ Worker pool  │ ─┘
                  └───────────┘               └──────┬───────┘
                                                     │
                                      ┌──────────────┴──────────────┐
                                      ▼                             ▼
                             Ticketmaster API                Resend (email)
```

A scheduler wakes on a fixed interval, asks the database which watched events are
due for a refresh, and pushes their IDs onto a buffered channel. A pool of workers
pulls from that channel, fetches the current state from the Ticketmaster Discovery
API, and hands the before/after pair to an evaluator.

The evaluator is the part that decides whether a human should be bothered. It's
**edge-triggered**: it compares the new snapshot against what the watch last saw
and only fires when something crossed a boundary. Otherwise every poll of an
on-sale event would send another email. It recognises a handful of transitions —
onsale date announced, presale open, public sale open, sale closed, cancelled,
rescheduled — and cancelled/rescheduled act as vetoes that suppress sale alerts.

Alerts are written to a `watch_alerts` table with a primary key that makes a
repeat insert a no-op, so a retry or a restart mid-send can't produce a duplicate
email. Everything a user was sent is visible in the UI's notification history.

The Ticketmaster API caps you at 5 req/s and 5,000 req/day, so all outbound
calls go through a shared rate limiter with a per-second token bucket and
a separate daily quota for polling versus interactive search — a busy poll loop
can't starve someone typing in the search box.

> One note on scope: Ticketmaster removed pricing data from the Discovery API in
> March 2025, and resale inventory is only visible to partner accounts. So this is
> an availability and on-sale watcher, not a price tracker.

## Tech stack

**Backend** — Go 1.25, [chi](https://github.com/go-chi/chi) for routing,
[pgx](https://github.com/jackc/pgx) for Postgres, [sqlc](https://sqlc.dev) to
generate type-safe query code from plain SQL, and
[goose](https://github.com/pressly/goose) for migrations (embedded in the binary
and applied on boot). Auth is JWT + bcrypt.

**Frontend** — React 18 + TypeScript, built with Vite. No router and no state
library; the app is small enough not to need either.

**Database** — PostgreSQL 16.

**Email** — [Resend](https://resend.com), on a DKIM/SPF-verified subdomain. With
no API key configured the app falls back to a log-only sender, so the whole
pipeline is demoable without an email account.

**Hosting** — [Fly.io](https://fly.io), single `shared-cpu-1x` machine in `ewr`,
with Postgres running alongside it. Deploys run from GitHub Actions on every push
to `main`.

**Packaging** — a multi-stage Dockerfile builds the SPA, embeds it into the Go
binary with `//go:embed`, and ships the result on `distroless/static`. One binary
serves both the API and the frontend; the final image has no shell and no runtime
dependencies.

The app keeps the scheduler and rate limiter in memory, so it deliberately runs as
exactly one instance. Horizontal scaling would need those moved into Postgres or
Redis first.

## Layout

```
cmd/api            entrypoint: wiring, graceful shutdown
internal/
  config           environment parsing
  httpapi          routes, handlers, middleware
  service          auth, search, watches, notifications, account
  ticketmaster     Discovery API client
  scheduler        picks due events, feeds the job queue
  worker           polls an event and reacts to the result
  evaluator        decides what is worth an alert (pure, no I/O)
  notifier         renders and sends email
  ratelimit        token bucket + daily quota + per-IP throttle
  store            sqlc-generated queries and migrations
web/               React SPA, embedded into the binary at build time
test/int_test      end-to-end tests against a real database
```

## Running locally

Requires Go 1.25+, Node 20+, Docker, and a
[Ticketmaster API key](https://developer.ticketmaster.com/my-apps).

```bash
cp .env.example .env      # then fill in TM_API_KEY
docker compose up -d db   # Postgres on :5432
make tidy

cd web && npm install && npm run build && cd ..
make run                  # http://localhost:8080
```

Migrations apply automatically on startup. Without `RESEND_API_KEY` set, emails
(including signup verification links) are printed to the log instead of sent —
grep for `[notify:email]`.

For frontend work, `npm run dev` in `web/` gives you Vite's dev server with hot
reload, proxied to the Go API.

Everything is configured through environment variables; see `.env.example` for the
ones that matter and `internal/config/config.go` for the full list with defaults.

## API

```
POST   /api/auth/register        POST   /api/auth/login
POST   /api/auth/forgot          POST   /api/auth/reset
GET    /api/auth/verify

GET    /api/search               GET    /api/me
POST   /api/watches              GET    /api/watches
PATCH  /api/watches/{id}         DELETE /api/watches/{id}
GET    /api/watches/{id}/history GET    /api/notifications
DELETE /api/account

GET    /healthz
```

Everything under `/api` except auth and unsubscribe requires a bearer token.
Unsubscribe links are stateless HMAC tokens, so they keep working from an old
email without a login.

## Tests

```bash
make test                    # unit
make test-int                # integration (needs `docker compose up -d db`)
cd web && npm test           # frontend
```
