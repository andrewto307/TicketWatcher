package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"ticket-watcher/internal/config"
	"ticket-watcher/internal/httpapi"
	"ticket-watcher/internal/ratelimit"
	"ticket-watcher/internal/scheduler"
	"ticket-watcher/internal/service"
	"ticket-watcher/internal/store"
	"ticket-watcher/internal/store/db"
	"ticket-watcher/internal/ticketmaster"
	"ticket-watcher/internal/worker"
)

const demoUserEmail = "demo@example.com"

func main() {
	// Load .env for local dev; ignored in Docker where env comes from env_file.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Apply migrations (throwaway database/sql connection).
	if err := store.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db pool: %v", err)
	}
	defer pool.Close()
	q := db.New(pool)

	user, err := q.GetUserByEmail(ctx, demoUserEmail)
	if err != nil {
		log.Fatalf("resolve demo user (did migrations seed it?): %v", err)
	}

	// Shared rate limiter: per-second token bucket + per-class daily quota.
	quota := ratelimit.NewDailyQuota(cfg.DailyBudgetPoll, cfg.DailyBudgetSearch)
	limiter := ratelimit.New(cfg.RatePerSec, cfg.RateBurst, quota)

	tm := ticketmaster.New(cfg.TMBaseURL, cfg.TMAPIKey, limiter)
	searchSvc := service.NewSearchService(tm)
	watchSvc := service.NewWatchService(q, tm, user.ID)

	// Background engine: scheduler -> jobs channel -> worker pool.
	jobs := make(chan int64, cfg.WorkerCount)
	var workersWG sync.WaitGroup
	worker.StartPool(ctx, cfg.WorkerCount, jobs, worker.Deps{Store: q, TM: tm}, &workersWG)

	sched := scheduler.New(q, quota, jobs, cfg.SchedulerInterval, cfg.MaxEventsPerTick)
	var schedWG sync.WaitGroup
	schedWG.Add(1)
	go func() { defer schedWG.Done(); sched.Run(ctx) }()

	// HTTP server.
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.NewRouter(searchSvc, watchSvc)}
	go func() {
		log.Printf("api listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	// Graceful shutdown: stop accepting HTTP, stop the scheduler, then drain workers.
	<-ctx.Done()
	log.Print("shutting down...")

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)

	schedWG.Wait() // scheduler fully stopped -> safe to close the jobs channel
	close(jobs)
	workersWG.Wait() // drain buffered/in-flight work
	log.Print("shutdown complete")
}
