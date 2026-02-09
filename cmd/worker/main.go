package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/notifications"
	"github.com/geocoder89/eventhub/internal/observability"
	"github.com/geocoder89/eventhub/internal/queue/worker"
	"github.com/geocoder89/eventhub/internal/repo/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1) init tracing first (so all spans/logs can attach)
	shutdownTracer, err := observability.InitTracer(context.Background(), "eventhub-worker", "localhost:4317")
	if err != nil {
		log.Fatalf("otel init failed: %v", err)
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	// 2) setup slog + trace handler (so logs include trace_id/span_id)
	base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(observability.NewTraceHandler(base))
	slog.SetDefault(logger)

	pool, err := pgxpool.New(ctx, cfg.DBURL)
	if err != nil {
		slog.Default().ErrorContext(ctx, "db connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Prom registry (NOTE: you still need to expose /metrics on worker if you want to scrape it)
	reg := prometheus.NewRegistry()
	prom := observability.NewProm(reg)

	jobsRepo := postgres.NewJobsRepo(pool, prom)
	eventsRepo := postgres.NewEventsRepo(pool, prom)

	host, _ := os.Hostname()
	workerID := host + "-" + strconv.Itoa(os.Getpid())

	healthAddr := os.Getenv("WORKER_HEALTH_ADDR")
	if healthAddr == "" {
		healthAddr = ":8081"
	}

	baseNotifier := notifications.NewLogNotifier()
	notifier := notifications.NewProtectedNotifier(baseNotifier, notifications.ProtectedNotifierConfig{
		Timeout:          2 * time.Second,
		FailureThreshold: 3,
		Cooldown:         15 * time.Second,
		HalfOpenMaxCalls: 1,
	})

	deliveriesRepo := postgres.NewNotificationsDeliveriesRepo(pool)

	w := worker.New(worker.Config{
		PollInterval:  2 * time.Second,
		WorkerID:      workerID,
		Concurrency:   1,
		ShutdownGrace: 10 * time.Second,
		LockTTL:       30 * time.Second,
		HealthAddr:    healthAddr,
	}, jobsRepo, eventsRepo, notifier, deliveriesRepo)

	slog.Default().InfoContext(ctx, "worker.start",
		"worker_id", workerID,
		"health_addr", healthAddr,
	)

	if err := w.Run(ctx); err != nil {
		slog.Default().ErrorContext(ctx, "worker.run_failed", "err", err)
	}

	slog.Default().InfoContext(context.Background(), "worker.shutdown_complete")
}
