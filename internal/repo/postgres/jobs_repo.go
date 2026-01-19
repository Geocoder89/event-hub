package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/geocoder89/eventhub/internal/domain/job"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type JobsRepo struct {
	pool *pgxpool.Pool
}

func NewJobsRepo(pool *pgxpool.Pool) *JobsRepo {
	return &JobsRepo{pool: pool}
}

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return false
}

func (r *JobsRepo) Create(ctx context.Context, req job.CreateRequest) (job.Job, error) {
	j := job.New(req)

	_, err := r.pool.Exec(ctx, `INSERT INTO jobs(
	 id, type, payload, status, attempts,max_attempts, run_at, locked_at, locked_by, last_error,idempotency_key, created_at, updated_at
	 ) VALUES (
		$1,$2,$3,$4,
		$5,$6,$7,$8,$9,
		$10,$11,$12,$13
	 
	 )
	 
	 `, j.ID, j.Type, j.Payload, string(j.Status), j.Attempts, j.MaxAttempts, j.RunAt, j.LockedAt, j.LockedBy, j.LastError, req.IdempotencyKey, j.CreatedAt, j.UpdatedAt)

	if err != nil {
		return job.Job{}, err
	}

	return j, nil
}

func (r *JobsRepo) CreateTx(ctx context.Context, tx pgx.Tx, req job.CreateRequest) (job.Job, error) {
	j := job.New(req)

	_, err := tx.Exec(ctx, `INSERT INTO jobs(
	 id, type, payload, status, attempts,max_attempts, run_at, locked_at, locked_by, last_error,idempotency_key, created_at, updated_at
	 ) VALUES (
		$1,$2,$3,$4,
		$5,$6,$7,$8,$9,
		$10,$11,$12,$13
	 
	 )
	 
	 `, j.ID, j.Type, j.Payload, string(j.Status), j.Attempts, j.MaxAttempts, j.RunAt, j.LockedAt, j.LockedBy, j.LastError, req.IdempotencyKey, j.CreatedAt, j.UpdatedAt)

	if err != nil {
		return job.Job{}, err
	}
	return j, nil
}

func (r *JobsRepo) MarkFailed(ctx context.Context, id string, errMsg string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'failed',
		    locked_at = NULL,
		    locked_by = NULL,
		    last_error = $2,
		    updated_at = NOW()
		WHERE id = $1
	`, id, errMsg)

	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return job.ErrJobNotFound
	}
	return nil
}
func (r *JobsRepo) MarkDone(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE jobs
		SET status = 'done',
			locked_at = NULL,
			locked_by = NULL,
			last_error = NULL,
			updated_at = NOW()
		WHERE id = $1
		`, id)

	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return job.ErrJobNotFound
	}
	return nil
}

func (r *JobsRepo) Reschedule(ctx context.Context, id string, runAt time.Time, errMsg string) error {

	// Useful for retries/backoff
	tag, err := r.pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'pending',
		    attempts = attempts + 1,
		    run_at = $2,
		    locked_at = NULL,
		    locked_by = NULL,
		    last_error = $3,
		    updated_at = NOW()
		WHERE id = $1
	`, id, runAt, errMsg)

	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return job.ErrJobNotFound
	}
	return nil
}

func (r *JobsRepo) ClaimNext(ctx context.Context, workerID string) (job.Job, error) {
	// Single statement claim using SKIP LOCKED pattern.
	// Only claims jobs ready to run (pending, run_at <= now), and not exceeded max_attempts.
	var j job.Job
	var status string

	err := r.pool.QueryRow(ctx, `
		WITH next AS (
			SELECT id
			FROM jobs
			WHERE status = 'pending'
			  AND run_at <= NOW()
			  AND attempts < max_attempts
			ORDER BY run_at ASC, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE jobs
		SET status = 'processing',
		    locked_at = NOW(),
		    locked_by = $1,
		    updated_at = NOW()
		WHERE id = (SELECT id FROM next)
		RETURNING id, type, payload, status,
		          attempts, max_attempts,
		          run_at, locked_at, locked_by,
		          last_error,idempotency_key, created_at, updated_at
	`, workerID).Scan(
		&j.ID, &j.Type, &j.Payload, &status,
		&j.Attempts, &j.MaxAttempts,
		&j.RunAt, &j.LockedAt, &j.LockedBy,
		&j.LastError, &j.IdempotencyKey, &j.CreatedAt, &j.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return job.Job{}, job.ErrJobNotFound // treat as “no job available”
		}
		return job.Job{}, err
	}

	j.Status = job.Status(status)
	return j, nil
}

func (r *JobsRepo) FetchNextPending(ctx context.Context) (job.Job, error) {
	var j job.Job
	var status string

	err := r.pool.QueryRow(ctx, `
		SELECT id, type, payload, status,
		       attempts, max_attempts,
		       run_at, locked_at, locked_by,
		       last_error,idempotency_key, created_at, updated_at
		FROM jobs
		WHERE status = 'pending'
		  AND run_at <= NOW()
		  AND attempts < max_attempts
		ORDER BY run_at ASC, created_at ASC
		LIMIT 1
	`).Scan(
		&j.ID, &j.Type, &j.Payload, &status,
		&j.Attempts, &j.MaxAttempts,
		&j.RunAt, &j.LockedAt, &j.LockedBy,
		&j.LastError, &j.IdempotencyKey, &j.CreatedAt, &j.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return job.Job{}, job.ErrJobNotFound // "nothing to do"
		}
		return job.Job{}, err
	}

	j.Status = job.Status(status)
	return j, nil
}

func (r *JobsRepo) GetByIdempotencyKey(ctx context.Context, key string) (job.Job, error) {
	var j job.Job
	var status string

	err := r.pool.QueryRow(ctx, `
		SELECT id, type, payload, status,
		       attempts, max_attempts,
		       run_at, locked_at, locked_by,
		       last_error, idempotency_key,
		       created_at, updated_at
		FROM jobs
		WHERE idempotency_key = $1
	`, key).Scan(
		&j.ID, &j.Type, &j.Payload, &status,
		&j.Attempts, &j.MaxAttempts,
		&j.RunAt, &j.LockedAt, &j.LockedBy,
		&j.LastError, &j.IdempotencyKey,
		&j.CreatedAt, &j.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return job.Job{}, job.ErrJobNotFound
		}
		return job.Job{}, err
	}

	j.Status = job.Status(status)
	return j, nil
}

// Admin ops endpoints

func (r *JobsRepo) List(ctx context.Context, status *string, limit, offset int) ([]job.Job, error) {
	q := `
		SELECT id, type, payload, status, attempts,
		max_attempts, run_at, locked_at, locked_by,
		last_error, idempotency_key, created_at, updated_at
		FROM jobs
		`

	args := []any{}
	if status != nil {
		q += " WHERE status = $1"
		args = append(args, *status)
		q += " ORDER BY updated_at DESC"
		q += " LIMIT $2 OFFSET $3"
		args = append(args, limit, offset)
	} else {
		q += " ORDER BY updated_at DESC LIMIT $1 OFFSET $2"
		args = append(args, limit, offset)
	}

	rows, err := r.pool.Query(ctx, q, args...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	out := make([]job.Job, 0)

	for rows.Next() {
		var j job.Job
		var st string

		err = rows.Scan(
			&j.ID, &j.Type, &j.Payload, &st,
			&j.Attempts, &j.MaxAttempts,
			&j.RunAt, &j.LockedAt, &j.LockedBy,
			&j.LastError, &j.IdempotencyKey,
			&j.CreatedAt, &j.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		j.Status = job.Status(st)

		out = append(out, j)
	}

	return out, rows.Err()
}

func (r *JobsRepo) GetByID(ctx context.Context, id string) (job.Job, error) {
	var j job.Job
	var status string

	err := r.pool.QueryRow(ctx, `
		SELECT id, type, payload, status,
		       attempts, max_attempts,
		       run_at, locked_at, locked_by,
		       last_error, idempotency_key,
		       created_at, updated_at
		FROM jobs
		WHERE id = $1
	`, id).Scan(
		&j.ID, &j.Type, &j.Payload, &status,
		&j.Attempts, &j.MaxAttempts,
		&j.RunAt, &j.LockedAt, &j.LockedBy,
		&j.LastError, &j.IdempotencyKey,
		&j.CreatedAt, &j.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return job.Job{}, job.ErrJobNotFound
		}
		return job.Job{}, err
	}

	j.Status = job.Status(status)
	return j, nil
}

func (r *JobsRepo) Retry(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'pending',
		    run_at = NOW(),
		    locked_at = NULL,
		    locked_by = NULL,
		    last_error = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'failed'
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return job.ErrJobNotFound
	}
	return nil
}
