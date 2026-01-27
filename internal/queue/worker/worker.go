package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/geocoder89/eventhub/internal/actorctx"
	"github.com/geocoder89/eventhub/internal/domain/job"
	notificationsdelivery "github.com/geocoder89/eventhub/internal/domain/notifications_delivery"
	"github.com/geocoder89/eventhub/internal/jobs"
	"github.com/geocoder89/eventhub/internal/notifications"
	"github.com/geocoder89/eventhub/internal/observability"
	"github.com/geocoder89/eventhub/internal/repo/postgres"
)

type publishPayload struct {
	EventID string `json:"eventId"`
}

type JobsRepository interface {
	ClaimNext(ctx context.Context, workerID string) (job.Job, error)
	// FetchNextPending(ctx context.Context) (job.Job, error)
	RequeueStaleProcessing(ctx context.Context, lockTTL time.Duration) (int64, error)
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
	LockTTL       time.Duration
}

type Worker struct {
	cfg        Config
	repo       JobsRepository
	events     EventsRepository
	metrics    *observability.JobMetrics
	notifier   notifications.Notifier
	deliveries *postgres.NotificationsDeliveriesRepo
	readyMu    sync.RWMutex
	ready      bool
}

func optional(v *string) string {
	if v == nil {
		return "null"
	}
	return *v
}

func New(cfg Config, repo JobsRepository, events EventsRepository, notifier notifications.Notifier, deliveries *postgres.NotificationsDeliveriesRepo,
) *Worker {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
	}

	if cfg.ShutdownGrace <= 0 {
		cfg.ShutdownGrace = 10 * time.Second
	}
	return &Worker{
		cfg:        cfg,
		repo:       repo,
		events:     events,
		metrics:    observability.NewJobMetrics(),
		notifier:   notifier,
		deliveries: deliveries,
		ready:      true,
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

func (w *Worker) requeueLoop(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-t.C:
			// short timeout for housekeeping
			hctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			n, err := w.repo.RequeueStaleProcessing(hctx, w.cfg.LockTTL)

			cancel()

			if err != nil {
				log.Printf("worker.requeue_stale error=%v", err)
				continue
			}
			if n > 0 {
				log.Printf("worker.requeue_stale count=%d", n)
			}
		}

	}
}

func (w *Worker) Run(ctx context.Context) error {
	// health server
	srv := &http.Server{Addr: ":8081", Handler: w.HealthHandler()}

	healthDone := make(chan struct{})

	go func() {
		log.Println("worker health server started on :8081")
		_ = srv.ListenAndServe()
		close(healthDone)
	}()

	// On shutdown: flip readiness -> keep alive briefly -> then shutdown server
	go func() {
		<-ctx.Done()

		w.readyMu.Lock()
		w.ready = false
		w.readyMu.Unlock()

		time.Sleep(5 * time.Second) // 503 observation window

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	// Worker loops
	jobsCh := make(chan job.Job)

	go w.logMetricsLoop(ctx, 30*time.Second)
	go w.requeueLoop(ctx)

	var wg sync.WaitGroup
	for i := 0; i < w.cfg.Concurrency; i++ {
		wg.Add(1)
		go func(workerNum int) {
			defer wg.Done()
			w.runWorker(ctx, workerNum, jobsCh)
		}(i + 1)
	}

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

	close(jobsCh)

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

	// IMPORTANT: keep process alive until health server finishes
	select {
	case <-healthDone:
	case <-time.After(7 * time.Second): // 5s window + 2s shutdown buffer
	}

	return nil
}

func (w *Worker) runWorker(ctx context.Context, workerNum int, jobsChan <-chan job.Job) {
	for j := range jobsChan {
		start := time.Now()

		uid := ""

		if j.UserID != nil {
			uid = *j.UserID
		}

		log.Printf(
			"job.start worker_num=%d job_id=%s job_type=%s user_id=%s attempts=%d/%d",
			workerNum, j.ID, j.Type, uid, j.Attempts, j.MaxAttempts,
		)

		execCtx := ctx

		if j.UserID != nil && *j.UserID != "" {
			execCtx = actorctx.WithUserID(execCtx, *j.UserID)
		}

		// Execute
		if err := w.execute(execCtx, j); err != nil {
			w.handleFailure(execCtx, j, err)
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
		if err := w.repo.MarkDone(execCtx, j.ID); err != nil {
			d := time.Since(start)
			w.metrics.ObserveDuration(d)
			w.metrics.IncFailed()

			log.Printf(
				"job.error worker=%d job=%s type=%s dur=%s err=%v",
				workerNum, j.ID, j.Type, d, err,
			)

			_ = w.repo.MarkFailed(execCtx, j.ID, "mark_done_failed: "+err.Error())
			continue

		}

		d := time.Since(start)
		w.metrics.ObserveDuration(d)
		w.metrics.IncDone()

		lockedBy := ""
		if j.LockedBy != nil {
			lockedBy = *j.LockedBy
		}

		log.Printf(
			"job.done worker_num=%d locked_by=%s job_id=%s job_type=%s user_id=%s duration_ms=%d",
			workerNum, lockedBy, j.ID, j.Type, optional(j.UserID), time.Since(start).Milliseconds(),
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

	case jobs.TypeRegistrationConfirmation:
		var p jobs.RegistrationConfirmationPayload
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}

		if w.notifier == nil {
			return fmt.Errorf("notifier not configured")
		}

		if w.deliveries == nil {
			return fmt.Errorf("deliveries repo not configured")
		}

		// Send-once gate

		err := w.deliveries.TryStartRegistration(ctx, j.ID, p.RegistrationID, p.Email)

		if err != nil {
			// Already sent == success (idempotent no-op)

			if errors.Is(err, notificationsdelivery.ErrAlreadySent) {
				return nil
			}

			// Another attempt is sending == retry later

			if errors.Is(err, notificationsdelivery.ErrInProgress) {
				return fmt.Errorf("confirmation send in progress")
			}

			return err
		}

		// Day 45: replaced initial log from day 43 with a notifier/email provider.
		err = w.notifier.SendRegistrationConfirmation(ctx, notifications.SendRegistrationConfirmationInput{
			Email:          p.Email,
			Name:           p.Name,
			EventID:        p.EventID,
			RegistrationID: p.RegistrationID,
		})

		if err != nil {
			// ALWAYS mark failed on any send error
			_ = w.deliveries.MarkRegistrationConfirmationFailed(
				ctx,
				p.RegistrationID,
				err.Error(),
			)

			if errors.Is(err, notifications.ErrCircuitOpen) {
				return fmt.Errorf("notifier fail-fast: %w", err)
			}

			return err
		}
		// 3) Mark sent
		if err := w.deliveries.MarkRegistrationConfirmationSent(ctx, p.RegistrationID, nil); err != nil {
			log.Printf("deliveries: mark sent failed reg=%s job=%s err=%v", p.RegistrationID, j.ID, err)
		}
		return nil

	case "test.crash":
		time.Sleep(60 * time.Second)

		return fmt.Errorf("unknown job type: %s", j.Type)

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
