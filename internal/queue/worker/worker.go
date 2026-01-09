package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/geocoder89/eventhub/internal/domain/job"
)

type JobsRepository interface {
	ClaimNext(ctx context.Context, workerID string) (job.Job, error)
	FetchNextPending(ctx context.Context) (job.Job, error)
	MarkDone(ctx context.Context, id string) error
}

type Config struct {
	PollInterval time.Duration
}

type Worker struct {
	cfg  Config
	repo JobsRepository
}

func New(cfg Config, repo JobsRepository) *Worker {
	return &Worker{
		cfg:  cfg,
		repo: repo,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("worker received shutdown signal")
			return nil

		case <-ticker.C:
			host, _ := os.Hostname()
			wID := fmt.Sprintf("%s-%d", host, os.Getpid())

			j, err := w.repo.ClaimNext(ctx, wID)

			if err != nil {
				if errors.Is(err, job.ErrJobNotFound) {
					continue
				}
				log.Printf("claim error: %v", err)
				continue
			}
			log.Printf("claimed job=%s locked_by=%s", j.ID, wID)

			// Day 35 placeholder: we mark it as done so it doesn't stay processing forever
			if err := w.repo.MarkDone(ctx, j.ID); err != nil {
				log.Printf("mark done error: %v", err)
			}
		}
	}
}
