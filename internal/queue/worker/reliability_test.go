package worker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/geocoder89/eventhub/internal/domain/job"
	"github.com/geocoder89/eventhub/internal/observability"
)

type fakeJobsRepo struct {
	claimNextFn              func(ctx context.Context, workerID string) (job.Job, error)
	requeueStaleProcessingFn func(ctx context.Context, lockTTL time.Duration) (int64, error)
	rescheduleFn             func(ctx context.Context, id string, runAt time.Time, errMsg string) error
	markFailedFn             func(ctx context.Context, id string, errMsg string) error
	markDoneFn               func(ctx context.Context, id string) error
}

func (f *fakeJobsRepo) ClaimNext(ctx context.Context, workerID string) (job.Job, error) {
	if f.claimNextFn != nil {
		return f.claimNextFn(ctx, workerID)
	}
	return job.Job{}, job.ErrJobNotFound
}

func (f *fakeJobsRepo) RequeueStaleProcessing(ctx context.Context, lockTTL time.Duration) (int64, error) {
	if f.requeueStaleProcessingFn != nil {
		return f.requeueStaleProcessingFn(ctx, lockTTL)
	}
	return 0, nil
}

func (f *fakeJobsRepo) Reschedule(ctx context.Context, id string, runAt time.Time, errMsg string) error {
	if f.rescheduleFn != nil {
		return f.rescheduleFn(ctx, id, runAt, errMsg)
	}
	return nil
}

func (f *fakeJobsRepo) MarkFailed(ctx context.Context, id string, errMsg string) error {
	if f.markFailedFn != nil {
		return f.markFailedFn(ctx, id, errMsg)
	}
	return nil
}

func (f *fakeJobsRepo) MarkDone(ctx context.Context, id string) error {
	if f.markDoneFn != nil {
		return f.markDoneFn(ctx, id)
	}
	return nil
}

type fakeEventsRepo struct {
	markPublishedFn func(ctx context.Context, eventID string) (bool, error)
}

func (f *fakeEventsRepo) MarkPublished(ctx context.Context, eventID string) (bool, error) {
	if f.markPublishedFn != nil {
		return f.markPublishedFn(ctx, eventID)
	}
	return true, nil
}

func TestHandleFailure_SchedulesRetryWhenAttemptsRemain(t *testing.T) {
	repo := &fakeJobsRepo{}
	metrics := observability.NewJobMetrics()

	rescheduled := 0
	var gotID string
	var gotErrMsg string
	var gotRunAt time.Time
	repo.rescheduleFn = func(ctx context.Context, id string, runAt time.Time, errMsg string) error {
		rescheduled++
		gotID = id
		gotErrMsg = errMsg
		gotRunAt = runAt
		return nil
	}

	markFailed := 0
	repo.markFailedFn = func(ctx context.Context, id string, errMsg string) error {
		markFailed++
		return nil
	}

	w := &Worker{repo: repo, metrics: metrics}
	j := job.Job{
		ID:          "job-1",
		Attempts:    0,
		MaxAttempts: 3,
	}

	before := time.Now().UTC()
	w.handleFailure(context.Background(), j, errors.New("boom"))

	if rescheduled != 1 {
		t.Fatalf("expected reschedule called once, got %d", rescheduled)
	}
	if markFailed != 0 {
		t.Fatalf("expected markFailed not called, got %d", markFailed)
	}
	if gotID != j.ID {
		t.Fatalf("unexpected reschedule id: %s", gotID)
	}
	if gotErrMsg != "boom" {
		t.Fatalf("unexpected err msg: %s", gotErrMsg)
	}
	if !gotRunAt.After(before) {
		t.Fatalf("expected runAt after now, got %s", gotRunAt)
	}

	s := metrics.Snapshot()
	if s.Retried != 1 {
		t.Fatalf("expected retried=1, got %d", s.Retried)
	}
	if s.DeadLettered != 0 {
		t.Fatalf("expected deadLettered=0, got %d", s.DeadLettered)
	}
}

func TestHandleFailure_DeadLettersWhenAttemptsExhausted(t *testing.T) {
	repo := &fakeJobsRepo{}
	metrics := observability.NewJobMetrics()

	rescheduled := 0
	repo.rescheduleFn = func(ctx context.Context, id string, runAt time.Time, errMsg string) error {
		rescheduled++
		return nil
	}

	markFailed := 0
	var gotErr string
	repo.markFailedFn = func(ctx context.Context, id string, errMsg string) error {
		markFailed++
		gotErr = errMsg
		return nil
	}

	w := &Worker{repo: repo, metrics: metrics}
	j := job.Job{
		ID:          "job-2",
		Attempts:    1,
		MaxAttempts: 2,
	}

	w.handleFailure(context.Background(), j, errors.New("nope"))

	if rescheduled != 0 {
		t.Fatalf("expected reschedule not called, got %d", rescheduled)
	}
	if markFailed != 1 {
		t.Fatalf("expected markFailed called once, got %d", markFailed)
	}
	if gotErr != "nope" {
		t.Fatalf("unexpected markFailed error msg: %s", gotErr)
	}

	s := metrics.Snapshot()
	if s.Retried != 0 {
		t.Fatalf("expected retried=0, got %d", s.Retried)
	}
	if s.DeadLettered != 1 {
		t.Fatalf("expected deadLettered=1, got %d", s.DeadLettered)
	}
}

func TestHandleFailure_RescheduleFailureFallsBackToMarkFailed(t *testing.T) {
	repo := &fakeJobsRepo{}
	metrics := observability.NewJobMetrics()

	rescheduled := 0
	repo.rescheduleFn = func(ctx context.Context, id string, runAt time.Time, errMsg string) error {
		rescheduled++
		return errors.New("db unavailable")
	}

	markFailed := 0
	var gotErr string
	repo.markFailedFn = func(ctx context.Context, id string, errMsg string) error {
		markFailed++
		gotErr = errMsg
		return nil
	}

	w := &Worker{repo: repo, metrics: metrics}
	j := job.Job{
		ID:          "job-3",
		Attempts:    0,
		MaxAttempts: 4,
	}

	w.handleFailure(context.Background(), j, errors.New("boom"))

	if rescheduled != 1 {
		t.Fatalf("expected reschedule called once, got %d", rescheduled)
	}
	if markFailed != 1 {
		t.Fatalf("expected markFailed called once, got %d", markFailed)
	}
	if !strings.HasPrefix(gotErr, "reschedule_failed: ") {
		t.Fatalf("expected reschedule_failed prefix, got %q", gotErr)
	}
	if !strings.HasSuffix(gotErr, "boom") {
		t.Fatalf("expected original error suffix, got %q", gotErr)
	}

	s := metrics.Snapshot()
	if s.Retried != 0 {
		t.Fatalf("expected retried=0, got %d", s.Retried)
	}
	if s.DeadLettered != 0 {
		t.Fatalf("expected deadLettered=0, got %d", s.DeadLettered)
	}
}

func TestProcessOne_NoClaimedJobReturnsFalseNil(t *testing.T) {
	repo := &fakeJobsRepo{}
	repo.claimNextFn = func(ctx context.Context, workerID string) (job.Job, error) {
		return job.Job{}, job.ErrJobNotFound
	}

	w := &Worker{
		cfg:    Config{WorkerID: "test-worker"},
		repo:   repo,
		events: &fakeEventsRepo{},
	}

	processed, err := w.ProcessOne(context.Background())
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if processed {
		t.Fatalf("expected processed=false when no jobs are available")
	}
}

func TestExponentialBackoff_StaysWithinExpectedBounds(t *testing.T) {
	tests := []struct {
		name    string
		attempt int
		base    time.Duration
	}{
		{name: "attempt 0", attempt: 0, base: 2 * time.Second},
		{name: "attempt 3", attempt: 3, base: 16 * time.Second},
		{name: "capped", attempt: 20, base: 5 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := ExponentialBackoff(tt.attempt)
			maxAllowed := tt.base + 250*time.Millisecond

			if delay < tt.base {
				t.Fatalf("delay below base: got=%s base=%s", delay, tt.base)
			}
			if delay > maxAllowed {
				t.Fatalf("delay above jitter cap: got=%s max=%s", delay, maxAllowed)
			}
		})
	}
}
