package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/geocoder89/eventhub/internal/domain/job"
)


type publishPayload struct {
	EventID string `json:"eventId"`
}

type JobsRepository interface {
	ClaimNext(ctx context.Context, workerID string) (job.Job, error)
	FetchNextPending(ctx context.Context) (job.Job, error)
	Reschedule(ctx context.Context, id string, runAt time.Time, errMsg string) error
	MarkFailed(ctx context.Context, id string, errMsg string) error
	MarkDone(ctx context.Context, id string) error
}

type EventsRepository interface {
	MarkPublished (ctx context.Context, eventID string) (bool, error)
}

type Config struct {
	PollInterval time.Duration
}

type Worker struct {
	cfg  Config
	repo JobsRepository
	events EventsRepository
}

func New(cfg Config, repo JobsRepository, events EventsRepository) *Worker {
	return &Worker{
		cfg:  cfg,
		repo: repo,
		events: events,
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

			

			err = w.execute(ctx,j) 

			if err != nil {
				w.handleFailure(ctx,j,err)
				continue
			}
			
			err = w.repo.MarkDone(ctx, j.ID)

			if err != nil {
				log.Printf("mark done error job=%s: %v", j.ID, err)
			} else {
				log.Printf("job done job=%s type=%s", j.ID, j.Type)
			}
		}
	}
}

func (w *Worker) execute(ctx context.Context, j job.Job) error {
	// simple implementation, the real behavior would be done in subsequent days.

	switch j.Type {
	case "event.publish":
			var p publishPayload
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}

		changed, err := w.events.MarkPublished(ctx, p.EventID)
		if err != nil {
			return err
		}
		if !changed {
			// already published => idempotent no-op
			return nil
		}

		// future: side effects like notifications/webhooks
		return nil

	default:
		return fmt.Errorf("unknown job type: %s", j.Type)
	}
}

func (w *Worker) handleFailure(ctx context.Context, j job.Job, execError error) {
	errMsg := execError.Error()

	// How many attempts will this failure represent?
	nextAttempt := j.Attempts + 1

	// if we have retries left, let us reschedule with exponential backoff

	if nextAttempt < j.MaxAttempts {
		delay := ExponentialBackoff(j.Attempts)
		runAt := time.Now().UTC().Add(delay)

		if err := w.repo.Reschedule(ctx, j.ID, runAt, errMsg); err != nil {
			log.Printf("reschedule error job=%s: %v", j.ID, err)
			_ = w.repo.MarkFailed(ctx, j.ID, "reschedule_failed: "+errMsg)
			return
		}

		log.Printf("job retry scheduled job=%s attempt=%d/%d next_run=%s err=%s",
			j.ID, nextAttempt, j.MaxAttempts, runAt.Format(time.RFC3339), errMsg)
		return
	}

	// Otherwise dead-letter it (status=failed + last_error)``
	if err := w.repo.MarkFailed(ctx, j.ID, errMsg); err != nil {
		log.Printf("mark failed error job=%s: %v", j.ID, err)
		return
	}

	log.Printf("job dead-lettered job=%s attempts=%d/%d err=%s",
		j.ID, nextAttempt, j.MaxAttempts, errMsg)


}
