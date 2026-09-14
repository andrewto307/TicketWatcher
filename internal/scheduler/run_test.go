package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"ticket-watcher/internal/ratelimit"
	"ticket-watcher/internal/store/db"
)

// Run is the loop the whole product hangs off: if it stalls, wedges, or dies
// quietly, every watch silently stops being polled and no error surfaces
// anywhere — the app looks healthy and simply never alerts anyone again.
//
// The recording lister below is concurrency-safe because Run drives tick from
// its own goroutine while the test asserts from another.

type recordingLister struct {
	mu     sync.Mutex
	calls  int
	limits []int32
	events []db.Event
	err    error
	// onCall, if set, is invoked (holding no lock) after each call — used to
	// observe ticks without polling.
	onCall func(n int)
}

func (r *recordingLister) ListDueEvents(_ context.Context, limit int32) ([]db.Event, error) {
	r.mu.Lock()
	r.calls++
	n := r.calls
	r.limits = append(r.limits, limit)
	evs, err := r.events, r.err
	r.mu.Unlock()

	if r.onCall != nil {
		r.onCall(n)
	}
	if err != nil {
		return nil, err
	}
	if int(limit) < len(evs) {
		return evs[:limit], nil
	}
	return evs, nil
}

func (r *recordingLister) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func (r *recordingLister) setErr(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

// runInBackground starts Run and returns a stop function that cancels it and
// waits for the goroutine to exit, failing the test if it does not.
func runInBackground(t *testing.T, s *Scheduler) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx)
	}()
	return func() {
		t.Helper()
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Run did not return after its context was cancelled; the process would hang on shutdown")
		}
	}
}

func TestRun_PollsRepeatedlyOnTheInterval(t *testing.T) {
	ticked := make(chan int, 8)
	lister := &recordingLister{
		events: makeEvents(2),
		onCall: func(n int) {
			select {
			case ticked <- n:
			default:
			}
		},
	}
	jobs := make(chan int64, 64)
	s := New(lister, ratelimit.NewDailyQuota(1000, 0), jobs, 5*time.Millisecond, 20)

	stop := runInBackground(t, s)
	defer stop()

	// Three separate ticks proves the loop repeats rather than running once.
	for i := 1; i <= 3; i++ {
		select {
		case <-ticked:
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d tick(s) happened; the loop stopped after the first pass", i-1)
		}
	}

	// ...and each pass must actually enqueue, not just wake up and return.
	deadline := time.After(2 * time.Second)
	for len(jobs) < 6 {
		select {
		case <-deadline:
			t.Fatalf("enqueued only %d job(s) across repeated ticks of 2 due events", len(jobs))
		case <-time.After(time.Millisecond):
		}
	}
}

func TestRun_DoesNotFireBeforeTheFirstInterval(t *testing.T) {
	lister := &recordingLister{events: makeEvents(1)}
	s := New(lister, ratelimit.NewDailyQuota(1000, 0), make(chan int64, 8), time.Hour, 20)

	stop := runInBackground(t, s)
	defer stop()

	// time.Ticker does not fire at t=0. This is deliberate: on boot the app has
	// just started its worker pool and DB pool, and a poll here would race them.
	time.Sleep(30 * time.Millisecond)
	if got := lister.callCount(); got != 0 {
		t.Errorf("%d poll(s) before the first interval elapsed", got)
	}
}

func TestRun_StopsPromptlyOnShutdown(t *testing.T) {
	lister := &recordingLister{events: makeEvents(1)}
	s := New(lister, ratelimit.NewDailyQuota(1000, 0), make(chan int64, 8), time.Hour, 20)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx) }()

	time.Sleep(10 * time.Millisecond) // let it reach the select
	start := time.Now()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run waited for the next tick instead of the cancellation")
	}
	// A one-hour interval must not become a one-hour shutdown.
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("shutdown took %v", elapsed)
	}
}

func TestRun_AlreadyCancelledContextDoesNoWork(t *testing.T) {
	lister := &recordingLister{events: makeEvents(3)}
	s := New(lister, ratelimit.NewDailyQuota(1000, 0), make(chan int64, 8), time.Millisecond, 20)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx) }()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return for an already-cancelled context")
	}
	if got := lister.callCount(); got != 0 {
		t.Errorf("%d poll(s) issued after shutdown had already been requested", got)
	}
}

// A database blip must not kill the loop. If Run returned on error, a single
// failed query during a deploy would permanently stop all polling with nothing
// but one log line to show for it.
func TestRun_SurvivesAListerError(t *testing.T) {
	ticked := make(chan int, 16)
	lister := &recordingLister{
		events: makeEvents(2),
		err:    errors.New("connection reset by peer"),
		onCall: func(n int) {
			select {
			case ticked <- n:
			default:
			}
		},
	}
	jobs := make(chan int64, 32)
	s := New(lister, ratelimit.NewDailyQuota(1000, 0), jobs, 5*time.Millisecond, 20)

	stop := runInBackground(t, s)
	defer stop()

	waitTick := func(what string) {
		t.Helper()
		select {
		case <-ticked:
		case <-time.After(2 * time.Second):
			t.Fatalf("no tick while %s", what)
		}
	}
	waitTick("the lister was failing")
	waitTick("the lister was failing")

	if got := len(jobs); got != 0 {
		t.Fatalf("enqueued %d job(s) from a failed query", got)
	}

	// Recovery: once the database comes back, polling resumes on the next tick.
	lister.setErr(nil)
	deadline := time.After(2 * time.Second)
	for len(jobs) == 0 {
		select {
		case <-deadline:
			t.Fatal("the loop never resumed after the lister recovered")
		case <-time.After(time.Millisecond):
		}
	}
}

// The Ticketmaster budget is shared with interactive search. When the poll
// budget is spent, the scheduler must not even ask the database for work —
// enqueuing jobs the workers cannot pay for would just burn DB queries.
func TestTick_ExhaustedBudgetSkipsTheQueryEntirely(t *testing.T) {
	lister := &recordingLister{events: makeEvents(5)}
	quota := ratelimit.NewDailyQuota(1, 0)
	if err := quota.Reserve(ratelimit.ClassPoll); err != nil { // spend the last one
		t.Fatal(err)
	}
	jobs := make(chan int64, 8)
	s := New(lister, quota, jobs, time.Minute, 20)

	s.tick(context.Background())

	if got := lister.callCount(); got != 0 {
		t.Errorf("queried the database %d time(s) with no budget left", got)
	}
	if got := len(jobs); got != 0 {
		t.Errorf("enqueued %d job(s) with no budget left", got)
	}
}

// tick must never block on the jobs channel. If it did, a saturated worker pool
// would freeze the scheduler goroutine, and because it is also the goroutine
// that watches ctx.Done, shutdown would hang with it.
func TestTick_DoesNotBlockOnASaturatedWorkerPool(t *testing.T) {
	lister := &recordingLister{events: makeEvents(10)}
	jobs := make(chan int64, 2) // pool already backed up
	s := New(lister, ratelimit.NewDailyQuota(1000, 0), jobs, time.Minute, 20)

	done := make(chan struct{})
	go func() { defer close(done); s.tick(context.Background()) }()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("tick blocked on a full jobs channel")
	}

	// It fills what it can and abandons the rest. Nothing is lost: the skipped
	// events keep their old last_polled_at, so the next ListDueEvents returns
	// them again.
	if got := len(jobs); got != 2 {
		t.Errorf("enqueued %d, want the channel's capacity of 2", got)
	}
}

// Shutdown must stop the enqueue loop even when the jobs channel has plenty of
// room. Without the ctx.Done arm, tick would happily push a full tick's worth of
// work at a worker pool that has already stopped draining, delaying exit.
//
// select picks uniformly among ready arms, so with a cancelled context each
// iteration has a 1/2 chance of taking the send. Over 200 events, getting
// through all of them by chance is a 2^-200 event; a version that ignores
// cancellation gets through every time.
func TestTick_CancellationStopsEnqueuingEvenWithRoomToSpare(t *testing.T) {
	const n = 200
	lister := &recordingLister{events: makeEvents(n)}
	jobs := make(chan int64, n*2) // never full
	s := New(lister, ratelimit.NewDailyQuota(10000, 0), jobs, time.Minute, n)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.tick(ctx)

	if got := len(jobs); got == n {
		t.Errorf("enqueued all %d events after shutdown was requested; cancellation is not checked while enqueuing", got)
	}
}

func TestTick_CancellationDuringEnqueueReturnsImmediately(t *testing.T) {
	lister := &recordingLister{events: makeEvents(10)}
	jobs := make(chan int64) // unbuffered and unread: every send would block
	s := New(lister, ratelimit.NewDailyQuota(1000, 0), jobs, time.Minute, 20)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() { defer close(done); s.tick(ctx) }()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("tick hung on an unbuffered jobs channel during shutdown")
	}
}

// The limit passed to the database is the smaller of the remaining budget and
// maxTick — never the larger, or a single tick could blow the whole daily quota.
func TestTick_LimitIsTheTighterOfBudgetAndMaxTick(t *testing.T) {
	tests := []struct {
		name      string
		budget    int
		maxTick   int
		wantLimit int32
	}{
		{"budget is tighter", 3, 20, 3},
		{"maxTick is tighter", 500, 7, 7},
		{"equal", 10, 10, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lister := &recordingLister{events: makeEvents(100)}
			s := New(lister, ratelimit.NewDailyQuota(tt.budget, 0), make(chan int64, 200), time.Minute, tt.maxTick)

			s.tick(context.Background())

			lister.mu.Lock()
			defer lister.mu.Unlock()
			if len(lister.limits) != 1 {
				t.Fatalf("got %d queries, want 1", len(lister.limits))
			}
			if lister.limits[0] != tt.wantLimit {
				t.Errorf("limit = %d, want %d", lister.limits[0], tt.wantLimit)
			}
		})
	}
}

func TestTick_NoDueEventsIsNotAnError(t *testing.T) {
	lister := &recordingLister{events: nil}
	jobs := make(chan int64, 4)
	s := New(lister, ratelimit.NewDailyQuota(1000, 0), jobs, time.Minute, 20)

	s.tick(context.Background()) // must not panic

	if got := len(jobs); got != 0 {
		t.Errorf("enqueued %d job(s) with nothing due", got)
	}
}

func TestTick_EnqueuesEveryEventIDExactlyOnce(t *testing.T) {
	lister := &recordingLister{events: makeEvents(5)}
	jobs := make(chan int64, 16)
	s := New(lister, ratelimit.NewDailyQuota(1000, 0), jobs, time.Minute, 20)

	s.tick(context.Background())

	seen := map[int64]int{}
	for _, id := range drain(jobs) {
		seen[id]++
	}
	for i := int64(1); i <= 5; i++ {
		if seen[i] != 1 {
			// A duplicate would double-charge the Ticketmaster budget for one event.
			t.Errorf("event %d enqueued %d time(s), want exactly 1", i, seen[i])
		}
	}
}
