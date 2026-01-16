package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/queue/worker"
	"github.com/geocoder89/eventhub/internal/repo/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DBURL)

	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}

	defer pool.Close()

	jobsRepo := postgres.NewJobsRepo(pool)
	eventsRepo := postgres.NewEventsRepo(pool)

	host, _ := os.Hostname()
	workerID := host + "-" + strconv.Itoa(os.Getpid())

	w := worker.New(worker.Config{
		PollInterval:  100 * time.Millisecond,
		WorkerID:      workerID,
		Concurrency:   4,
		ShutdownGrace: 10 * time.Second,
	}, jobsRepo, eventsRepo)

	log.Println("worker has started")

	if err := w.Run(ctx); err != nil {
		log.Printf("worker stopped with error: %v", err)
	}

	log.Println("worker shutdown complete")

}
