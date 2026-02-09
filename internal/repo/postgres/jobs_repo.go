package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/geocoder89/eventhub/internal/domain/job"
	"github.com/geocoder89/eventhub/internal/observability"
	"github.com/geocoder89/eventhub/internal/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrJobNotFailed = errors.New("job is not failed")

type JobsRepo struct {
	pool *pgxpool.Pool
	prom *observability.Prom
}

func (repo *JobsRepo) observe(op string, fn func() error) error {
	if repo.prom != nil {
		return repo.prom.ObserveDB(op, fn)
	}
	return fn()
}

func NewJobsRepo(pool *pgxpool.Pool, prom *observability.Prom) *JobsRepo {
	return &JobsRepo{pool: pool, prom: prom}
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
	op := "jobs.create"

	var err error

	err = r.observe(op, func() error {
		_, err = r.pool.Exec(ctx, `INSERT INTO jobs(
	 id, type, payload, status, attempts,max_attempts, run_at, locked_at, locked_by, last_error,idempotency_key,priority,user_id, created_at, updated_at
	 ) VALUES (
		$1,$2,$3,$4,
		$5,$6,$7,$8,$9,
		$10,$11,$12,$13,$14,$15
	 
	 )
	 
	 `, j.ID, j.Type, j.Payload, string(j.Status), j.Attempts, j.MaxAttempts, j.RunAt, j.LockedAt, j.LockedBy, j.LastError, req.IdempotencyKey, j.Priority, j.UserID, j.CreatedAt, j.UpdatedAt)

		return err
	})

	if err != nil {
		return job.Job{}, err
	}

	return j, nil
}

func (r *JobsRepo) CreateTx(ctx context.Context, tx pgx.Tx, req job.CreateRequest) (job.Job, error) {
	j := job.New(req)

	op := "jobs.create_tx"
	var err error

	err = r.observe(
		op, func() error {

			_, err = tx.Exec(ctx, `INSERT INTO jobs(
	 id, type, payload, status, attempts,max_attempts, run_at, locked_at, locked_by, last_error,idempotency_key,priority,user_id, created_at, updated_at
	 ) VALUES (
		$1,$2,$3,$4,
		$5,$6,$7,$8,$9,
		$10,$11,$12,$13,$14,$15
	 
	 )
	 
	 `, j.ID, j.Type, j.Payload, string(j.Status), j.Attempts, j.MaxAttempts, j.RunAt, j.LockedAt, j.LockedBy, j.LastError, req.IdempotencyKey, j.Priority, j.UserID, j.CreatedAt, j.UpdatedAt)
			return err
		},
	)

	if err != nil {
		return job.Job{}, err
	}
	return j, nil
}

func (r *JobsRepo) MarkFailed(ctx context.Context, id string, errMsg string) error {
	var tag pgconn.CommandTag
	var err error
	op := "jobs.mark_failed"

	err = r.observe(op, func() error {
		tag, err = r.pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'failed',
		    locked_at = NULL,
		    locked_by = NULL,
		    last_error = $2,
		    updated_at = NOW()
		WHERE id = $1
	`, id, errMsg)
		return err
	})

	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return job.ErrJobNotFound
	}
	return nil
}
func (r *JobsRepo) MarkDone(ctx context.Context, id string) error {
	var tag pgconn.CommandTag
	var err error
	op := "jobs.mark_done"

	err = r.observe(op, func() error {

		tag, err = r.pool.Exec(ctx,
			`UPDATE jobs
		SET status = 'done',
			locked_at = NULL,
			locked_by = NULL,
			last_error = NULL,
			updated_at = NOW()
		WHERE id = $1
		`, id)

		return err
	})

	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return job.ErrJobNotFound
	}
	return nil
}

func (r *JobsRepo) Reschedule(ctx context.Context, id string, runAt time.Time, errMsg string) error {
	var tag pgconn.CommandTag
	var err error

	op := "jobs.reschedule"

	err = r.observe(op, func() error {
		// Useful for retries/backoff
		tag, err = r.pool.Exec(ctx, `
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

		return err

	})

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
	var err error

	op := "jobs.claim_next"

	err = r.observe(op, func() error {
		return r.pool.QueryRow(ctx, `
		WITH next AS (
			SELECT id
			FROM jobs
			WHERE status = 'pending'
			  AND run_at <= NOW()
			  AND attempts < max_attempts
			ORDER BY priority DESC, run_at ASC, created_at ASC
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
		          last_error,idempotency_key,priority,user_id, created_at, updated_at
	`, workerID).Scan(
			&j.ID, &j.Type, &j.Payload, &status,
			&j.Attempts, &j.MaxAttempts,
			&j.RunAt, &j.LockedAt, &j.LockedBy,
			&j.LastError, &j.IdempotencyKey, &j.Priority, &j.UserID, &j.CreatedAt, &j.UpdatedAt,
		)

	})

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

	var err error

	op := "jobs.fetch_next_pending"

	err = r.observe(op, func() error {

		return r.pool.QueryRow(ctx, `
		SELECT id, type, payload, status,
		       attempts, max_attempts,
		       run_at, locked_at, locked_by,
		       last_error,idempotency_key,priority,user_id, created_at, updated_at
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
			&j.LastError, &j.IdempotencyKey, &j.Priority, &j.UserID, &j.CreatedAt, &j.UpdatedAt,
		)
	})

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
	var err error
	op := "jobs.get_by_idempotency_key"

	err = r.observe(op, func() error {
		return r.pool.QueryRow(ctx, `
		SELECT id, type, payload, status,
		       attempts, max_attempts,
		       run_at, locked_at, locked_by,
		       last_error, idempotency_key,priority,user_id,
		       created_at, updated_at
		FROM jobs
		WHERE idempotency_key = $1
	`, key).Scan(
			&j.ID, &j.Type, &j.Payload, &status,
			&j.Attempts, &j.MaxAttempts,
			&j.RunAt, &j.LockedAt, &j.LockedBy,
			&j.LastError, &j.IdempotencyKey, &j.Priority, &j.UserID,
			&j.CreatedAt, &j.UpdatedAt,
		)

	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return job.Job{}, job.ErrJobNotFound
		}
		return job.Job{}, err
	}

	j.Status = job.Status(status)
	return j, nil
}

// Requeue a stale processing job i.e lockTTL is greater than the time now i.e it is stale

func (r *JobsRepo) RequeueStaleProcessing(ctx context.Context, lockTTL time.Duration) (int64, error) {
	secs := int64(lockTTL.Seconds())
	if secs <= 0 {
		secs = 30
	}
	var rows int64
	var err error

	op := "jobs.requeue_stale"
	err = r.observe(op, func() error {
		tag, err := r.pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'pending',
		    locked_at = NULL,
		    locked_by = NULL,
		    updated_at = NOW()
		WHERE status = 'processing'
		  AND locked_at IS NOT NULL
		  AND locked_at < NOW() - ($1 * INTERVAL '1 second')
	`, secs)

		if err != nil {
			return err
		}
		rows = tag.RowsAffected()
		return nil

	})

	return rows, err

}

// Admin ops endpoints

func (r *JobsRepo) ListCursor(
	ctx context.Context,
	status *string,
	limit int,
	afterUpdatedAt time.Time,
	afterID string,
) (items []job.Job, nextCursor *string, hasMore bool, err error) {
	op := "jobs.admin.list_cursor"

	base := `
		SELECT id, type, payload, status, attempts,
		       max_attempts, run_at, locked_at, locked_by,
		       last_error, idempotency_key, priority, user_id,
		       created_at, updated_at
		FROM jobs
	`

	var (
		conds   []string
		args    []any
		argsPos = 1
	)

	if status != nil {
		conds = append(conds, fmt.Sprintf("status = $%d", argsPos))
		args = append(args, *status)
		argsPos++
	}

	// DESC keyset: fetch rows "older" than cursor
	conds = append(conds, fmt.Sprintf("(updated_at, id) < ($%d, $%d)", argsPos, argsPos+1))
	args = append(args, afterUpdatedAt, afterID)
	argsPos += 2

	q := base
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}

	limitPlusOne := limit + 1
	q += fmt.Sprintf(" ORDER BY updated_at DESC, id DESC LIMIT $%d", argsPos)
	args = append(args, limitPlusOne)

	var (
		rows pgx.Rows
	)

	err = r.observe(op, func() error {
		var qerr error
		rows, qerr = r.pool.Query(ctx, q, args...)
		return qerr
	})
	if err != nil {
		return nil, nil, false, err
	}
	defer rows.Close()

	out := make([]job.Job, 0, limit)

	for rows.Next() {
		var j job.Job
		var st string

		if scanErr := rows.Scan(
			&j.ID, &j.Type, &j.Payload, &st,
			&j.Attempts, &j.MaxAttempts,
			&j.RunAt, &j.LockedAt, &j.LockedBy,
			&j.LastError, &j.IdempotencyKey, &j.Priority, &j.UserID,
			&j.CreatedAt, &j.UpdatedAt,
		); scanErr != nil {
			return nil, nil, false, scanErr
		}
		j.Status = job.Status(st)
		out = append(out, j)
	}

	if rows.Err() != nil {
		return nil, nil, false, rows.Err()
	}

	if len(out) > limit {
		hasMore = true
		out = out[:limit]
		last := out[len(out)-1]

		cur, encErr := utils.EncodeJobCursor(last.UpdatedAt, last.ID)
		if encErr != nil {
			return nil, nil, false, encErr
		}
		nextCursor = &cur
	}

	return out, nextCursor, hasMore, nil
}

func (r *JobsRepo) GetByID(ctx context.Context, id string) (job.Job, error) {
	var j job.Job
	var status string
	var err error
	op := "jobs.admin.get_by_id"

	err = r.observe(op, func() error {

		return r.pool.QueryRow(ctx, `
		SELECT id, type, payload, status,
		       attempts, max_attempts,
		       run_at, locked_at, locked_by,
		       last_error, idempotency_key,priority,user_id,
		       created_at, updated_at
		FROM jobs
		WHERE id = $1
	`, id).Scan(
			&j.ID, &j.Type, &j.Payload, &status,
			&j.Attempts, &j.MaxAttempts,
			&j.RunAt, &j.LockedAt, &j.LockedBy,
			&j.LastError, &j.IdempotencyKey, &j.Priority, &j.UserID,
			&j.CreatedAt, &j.UpdatedAt,
		)
	})
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
	// check job exists + status
	var status string

	var err error
	op := "jobs.admin.retry.check_status"

	err = r.observe(op, func() error {
		return r.pool.QueryRow(ctx, `SELECT status FROM jobs WHERE id = $1`, id).Scan(&status)
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return job.ErrJobNotFound
		}
		return err
	}

	if status != "failed" {
		return ErrJobNotFailed
	}

	// 2) requeue

	requeueOp := "jobs.admin.retry.requeue"

	requeueFn := func() error {
		_, e := r.pool.Exec(ctx, `
		UPDATE jobs
		SET status = 'pending',
		    run_at = NOW(),
		    locked_at = NULL,
		    locked_by = NULL,
		    last_error = NULL,
		    updated_at = NOW()
		WHERE id = $1
	`, id)
		return e
	}

	return r.observe(requeueOp, requeueFn)

}

// POST /admin/jobs/reprocess-dead?limit=50
func (r *JobsRepo) RetryManyFailed(ctx context.Context, limit int) (int64, error) {
	var tag pgconn.CommandTag
	op := "jobs.admin.retry_many_failed"
	var err error

	if limit <= 0 {
		limit = 50
	}

	if limit > 500 {
		limit = 500

	}

	fn := func() error {
		tag, err = r.pool.Exec(ctx,
			`
		WITH picked AS (
			SELECT id
			FROM jobs
			WHERE status = 'failed'
			ORDER BY updated_at DESC
			LIMIT $1
		)
		UPDATE jobs
		SET status = 'pending',
		    run_at = NOW(),
		    locked_at = NULL,
		    locked_by = NULL,
		    last_error = NULL,
		    updated_at = NOW()
		WHERE id IN (SELECT id FROM picked)
		`, limit)

		return err
	}

	err = r.observe(op, fn)
	if err != nil {
		return 0, err
	}

	return tag.RowsAffected(), nil

}
