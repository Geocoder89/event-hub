package worker

import (
	"context"
	"errors"
	"time"

	"github.com/geocoder89/eventhub/internal/domain/job"
)

func (w *Worker) ProcessOne(ctx context.Context) (bool, error) {

	claimCtx, cancel := context.WithTimeout(ctx, 2*time.Second)

	j, err := w.repo.ClaimNext(claimCtx, w.cfg.WorkerID)
	cancel()

	if err != nil {
		if errors.Is(err, job.ErrJobNotFound) {
			return false, nil
		}

		return false, err
	}

	err = w.execute(ctx, j)

	if err != nil {
		w.handleFailure(ctx, j, err)
		return true, nil
	}

	err = w.repo.MarkDone(ctx, j.ID)

	if err != nil {
		_ = w.repo.MarkFailed(ctx, j.ID, "mark_done_failed: "+err.Error())
		return true, err
	}

	return true, nil
}
