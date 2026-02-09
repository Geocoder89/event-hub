package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/geocoder89/eventhub/internal/domain/event"
	"github.com/geocoder89/eventhub/internal/domain/registration"
	"github.com/geocoder89/eventhub/internal/observability"
	"github.com/geocoder89/eventhub/internal/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RegistrationRepo struct {
	pool *pgxpool.Pool
	prom *observability.Prom
}

func NewRegistrationsRepo(pool *pgxpool.Pool, prom *observability.Prom) *RegistrationRepo {
	return &RegistrationRepo{
		pool: pool,
		prom: prom,
	}
}

func (repo *RegistrationRepo) observe(op string, fn func() error) error {
	if repo.prom != nil {

		return repo.prom.ObserveDB(op, fn)
	}
	return fn()
}

func (repo *RegistrationRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return repo.pool.BeginTx(ctx, pgx.TxOptions{})
}

func (repo *RegistrationRepo) CreateTx(ctx context.Context, tx pgx.Tx, req registration.CreateRegistrationRequest) (reg registration.Registration, err error) {
	// check duplicate emails for events

	var exists bool

	err = repo.observe("registrations.create_tx.duplicate_check", func() error {
		return tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM registrations
			WHERE event_id = $1 AND email = $2
		)`, req.EventID, req.Email).Scan(&exists)
	})

	if err != nil {
		return
	}

	if exists {
		err = registration.ErrAlreadyRegistered
		return
	}

	// 2) lock event row + check capacity
	var capacity int
	var current int
	err = repo.observe("registrations.create_tx.capacity_lock", func() error {
		return tx.QueryRow(ctx, `
		SELECT e.capacity,
			(SELECT COUNT(*) FROM registrations r WHERE r.event_id = e.id) AS current
		FROM events e
		WHERE e.id = $1
		FOR UPDATE
	`, req.EventID).Scan(&capacity, &current)
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = event.ErrNotFound
		}

		return
	}

	if current >= capacity {
		err = registration.ErrEventFull
		return
	}

	reg = registration.NewFromCreateRequest(req)

	err = repo.observe("registrations.create_tx.insert", func() error {
		_, e := tx.Exec(ctx, `
		INSERT INTO registrations (id, event_id, user_id, name, email, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, reg.ID, reg.EventID, reg.UserID, reg.Name, reg.Email, reg.CreatedAt, reg.UpdatedAt)
		return e
	})

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "registrations_event_email_uniq" {
			err = registration.ErrAlreadyRegistered
			return
		}
		return
	}

	return
}

// implementation of the create method using the idiomatic Go "named return and defer" approach
func (repo *RegistrationRepo) Create(ctx context.Context, req registration.CreateRegistrationRequest) (reg registration.Registration, err error) {
	// Enforce capacity and uniqueness into a single transaction

	tx, err := repo.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	reg, err = repo.CreateTx(ctx, tx, req)

	if err != nil {
		return
	}

	err = tx.Commit(ctx)

	if err != nil {
		return
	}

	// success: registration is set err == nil
	return

	/* OLDER IMPLEMENTATION OF CREATE REGISTRATION WITHOUT DB LOCK VIA TRANSACTIONS.
	 */
	// reg := registration.NewFromCreateRequest(req)

	// _, err := repo.pool.Exec(ctx,
	// 	`INSERT INTO registrations (id,event_id,name,email, created_at, updated_at)
	// 	 VALUES ($1,$2,$3,$4,$5,$6)
	// 	`,
	// 	reg.ID, reg.EventID,reg.Name, reg.Email,reg.CreatedAt,reg.UpdatedAt,
	// )

	// if err != nil {
	// 	var pgErr *pgconn.PgError
	// 	if errors.As(err,&pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "registrations_event_email_uniq" {
	// 		return registration.Registration{}, registration.ErrAlreadyRegistered

	// 	}
	// 	return registration.Registration{},err
	// }
	// return reg, nil
}

func (repo *RegistrationRepo) ListByEvent(ctx context.Context, eventID string) (regs []registration.Registration, err error) {
	var rows pgx.Rows

	err = repo.observe("registrations.list_by_event", func() error {
		rows, err = repo.pool.Query(ctx,
			`
	SELECT id, event_id,user_id,name, email, created_at,updated_at
	FROM registrations
	WHERE event_id = $1
	ORDER BY created_at ASC, id ASC
	`,
			eventID,
		)
		return err
	})

	if err != nil {
		return
	}

	defer rows.Close()

	regs = make([]registration.Registration, 0)

	for rows.Next() {
		var r registration.Registration

		e := rows.Scan(&r.ID, &r.EventID, &r.UserID, &r.Name, &r.Email, &r.CreatedAt, &r.UpdatedAt)

		if e != nil {
			err = e
			return
		}
		regs = append(regs, r)
	}

	e := rows.Err()

	if e != nil {
		if repo.prom != nil {
			repo.prom.DbErrorsTotal.WithLabelValues("registrations.list_by_event", "rows_err").Inc()
		}
		err = e
		return
	}

	// in the event i want a 404 if the event itself does not exist

	if len(regs) == 0 {
		// check if event exists at all
		var dummy string

		err = repo.observe("registrations.list_by_event.check_event_exists", func() error {
			return repo.pool.QueryRow(ctx, `SELECT id FROM events WHERE id = $1`, eventID).Scan(&dummy)
		})

		if errors.Is(err, pgx.ErrNoRows) {
			err = event.ErrNotFound

			return
		}

		if err != nil {
			return
		}
	}

	return
}

func (repo *RegistrationRepo) CountForEvent(ctx context.Context, eventID string) (int, error) {
	op := "registrations.count_for_event"
	var total int
	err := repo.observe(op, func() error {
		return repo.pool.QueryRow(ctx, `SELECT COUNT(*) FROM registrations WHERE event_id = $1`, eventID).Scan(&total)
	})
	return total, err
}

func (repo *RegistrationRepo) ListByEventCursor(
	ctx context.Context,
	eventID string,
	limit int,
	afterCreatedAt time.Time,
	afterID string,
) (items []registration.Registration, nextCursor *string, hasMore bool, err error) {
	op := "registrations.list_by_event_cursor"

	q := `
		SELECT id, event_id, user_id, name, email, created_at, updated_at
		FROM registrations
		WHERE event_id = $1
		  AND (created_at, id) > ($2, $3)
		ORDER BY created_at ASC, id ASC
		LIMIT $4
	`
	limitPlusOne := limit + 1

	var rows pgx.Rows
	err = repo.observe(op, func() error {
		var qerr error
		rows, qerr = repo.pool.Query(ctx, q, eventID, afterCreatedAt, afterID, limitPlusOne)
		return qerr
	})
	if err != nil {
		return nil, nil, false, err
	}
	defer rows.Close()

	out := make([]registration.Registration, 0, limit)

	for rows.Next() {
		var r registration.Registration
		if scanErr := rows.Scan(&r.ID, &r.EventID, &r.UserID, &r.Name, &r.Email, &r.CreatedAt, &r.UpdatedAt); scanErr != nil {
			return nil, nil, false, scanErr
		}
		out = append(out, r)
	}
	if rows.Err() != nil {
		return nil, nil, false, rows.Err()
	}

	if len(out) > limit {
		hasMore = true
		out = out[:limit]
		last := out[len(out)-1]
		cur, encErr := utils.EncodeRegistrationCursor(last.CreatedAt, last.ID)
		if encErr != nil {
			return nil, nil, false, encErr
		}
		nextCursor = &cur
	}

	return out, nextCursor, hasMore, nil
}

func (repo *RegistrationRepo) GetByID(ctx context.Context, eventID, registrationID string) (foundReg registration.Registration, newErr error) {
	var r registration.Registration
	err := repo.observe("registrations.get_by_id", func() error {
		return repo.pool.QueryRow(ctx,
			`
		SELECT id, event_id, user_id, name, email, created_at, updated_at
		FROM registrations
		WHERE id = $1 AND event_id = $2
		`,
			registrationID, eventID,
		).Scan(&r.ID, &r.EventID, &r.UserID, &r.Name, &r.Email, &r.CreatedAt, &r.UpdatedAt)
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			newErr = registration.ErrNotFound
			return
		}

		newErr = err
		return
	}

	foundReg = r
	return
}

// Delete removes a single registration for an event

func (repo *RegistrationRepo) Delete(ctx context.Context, eventID, registrationID string) (err error) {

	var tag pgconn.CommandTag
	op := "registrations.delete"
	err = repo.observe(op, func() error {
		var err error
		tag, err = repo.pool.Exec(ctx, `DELETE FROM registrations WHERE id = $1 AND event_id = $2`, registrationID, eventID)

		return err
	})

	if err != nil {
		return
	}

	if tag.RowsAffected() == 0 {
		err = registration.ErrNotFound

		return
	}

	return
}
