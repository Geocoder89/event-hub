package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/geocoder89/eventhub/internal/domain/job"
	"github.com/geocoder89/eventhub/internal/observability"
)

type publishPayload struct {
	EventID string `json:"eventId"`
}

type JobsRepository interface {
	ClaimNext(ctx context.Context, workerID string) (job.Job, error)
	// FetchNextPending(ctx context.Context) (job.Job, error)
	Reschedule(ctx context.Context, id string, runAt time.Time, errMsg string) error
	MarkFailed(ctx context.Context, id string, errMsg string) error
	MarkDone(ctx context.Context, id string) error
}

type EventsRepository interface {
	MarkPublished(ctx context.Context, eventID string) (bool, error)
}

type Config struct {
	PollInterval  time.Duration
	WorkerID      string
	Concurrency   int // concurrency control
	ShutdownGrace time.Duration
}

type Worker struct {
	cfg     Config
	repo    JobsRepository
	events  EventsRepository
	metrics *observability.JobMetrics
}

func New(cfg Config, repo JobsRepository, events EventsRepository) *Worker {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
	}

	if cfg.ShutdownGrace <= 0 {
		cfg.ShutdownGrace = 10 * time.Second
	}
	return &Worker{
		cfg:     cfg,
		repo:    repo,
		events:  events,
		metrics: observability.NewJobMetrics(),
	}
}

func (w *Worker) logMetricsLoop(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)

	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-t.C:
			s := w.metrics.Snapshot()
			log.Printf(
				"job metrics claimed=%d done=%d failed=%d retried=%d dlq=%d duration_count=%d dur_avg=%s duration_max=%s",
				s.Claimed, s.Done, s.Failed, s.Retried, s.DeadLettered, s.DurationCount, s.AverageDuration, s.MaxDuration,
			)
		}
	}
}

func (w *Worker) Run(ctx context.Context) error {
	// job channel is the work queue inside the worker process
	jobsCh := make(chan job.Job)

	// start a metrics goroutine
	go w.logMetricsLoop(ctx, 30*time.Second)

	// Start N worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < w.cfg.Concurrency; i++ {
		wg.Add(1)
		go func(workerNum int) {
			defer wg.Done()
			w.runWorker(ctx, workerNum, jobsCh)
		}(i + 1)
	}

	// Producer loop: claim jobs and feed the pool
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

producerLoop:
	for {
		select {
		case <-ctx.Done():
			log.Println("worker: shutdown signal received; stopping claims")
			break producerLoop

		case <-ticker.C:
			for i := 0; i < w.cfg.Concurrency; i++ {
				claimCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				j, err := w.repo.ClaimNext(claimCtx, w.cfg.WorkerID)
				cancel()

				if err != nil {
					if errors.Is(err, job.ErrJobNotFound) {
						break
					}
					log.Printf("worker: claim error: %v", err)
					break
				}

				select {
				case jobsCh <- j:
					if w.metrics != nil {
						w.metrics.IncClaimed()
					}
				case <-ctx.Done():
					break producerLoop
				}
			}
		}
	}

	// Stop accepting new jobs, let workers drain
	close(jobsCh)

	// Wait for in-flight jobs with grace timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("worker: all in-flight jobs completed")
	case <-time.After(w.cfg.ShutdownGrace):
		log.Printf("worker: shutdown grace (%s) exceeded; exiting", w.cfg.ShutdownGrace)
	}

	return nil
}

func (w *Worker) runWorker(ctx context.Context, workerNum int, jobsChan <-chan job.Job) {
	for j := range jobsChan {
		start := time.Now()

		log.Printf(
			"job.start worker=%d job=%s type=%s attempts=%d/%d",
			workerNum, j.ID, j.Type, j.Attempts, j.MaxAttempts,
		)

		// Execute
		if err := w.execute(ctx, j); err != nil {
			w.handleFailure(ctx, j, err)
			d := time.Since(start)
			w.metrics.ObserveDuration(d)
			w.metrics.IncFailed()
			log.Printf(
				"job.error worker=%d job=%s type=%s dur=%s err=%v",
				workerNum, j.ID, j.Type, d, err,
			)
			continue
		}

		// Mark done
		if err := w.repo.MarkDone(ctx, j.ID); err != nil {
			d := time.Since(start)
			w.metrics.ObserveDuration(d)
			w.metrics.IncFailed()

			log.Printf(
				"job.error worker=%d job=%s type=%s dur=%s err=%v",
				workerNum, j.ID, j.Type, d, err,
			)

			_ = w.repo.MarkFailed(ctx, j.ID, "mark_done_failed: "+err.Error())
			continue

		}

		d := time.Since(start)
		w.metrics.ObserveDuration(d)
		w.metrics.IncDone()

		log.Printf(
			"job.done worker=%d job=%s type=%s dur=%s",
			workerNum, j.ID, j.Type, d,
		)
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
		time.Sleep(750 * time.Millisecond)
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

		if w.metrics != nil {
			w.metrics.IncRetried()
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

	if w.metrics != nil {
		w.metrics.IncDeadLettered()
	}

	log.Printf("job dead-lettered job=%s attempts=%d/%d err=%s",
		j.ID, nextAttempt, j.MaxAttempts, errMsg)

}
