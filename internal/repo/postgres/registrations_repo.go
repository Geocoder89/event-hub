package postgres

import (
	"context"
	"errors"

	"github.com/geocoder89/eventhub/internal/domain/event"
	"github.com/geocoder89/eventhub/internal/domain/registration"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RegistrationRepo struct {
	pool *pgxpool.Pool
}

func NewRegistrationsRepo(pool *pgxpool.Pool) *RegistrationRepo {
	return &RegistrationRepo{
		pool: pool,
	}
}

// implementation of the create method using the idiomatic Go "named return and defer" approach 
func (repo *RegistrationRepo) Create(ctx context.Context, req registration.CreateRegistrationRequest) (reg registration.Registration, err error) {
	// Enforce capacity and uniqueness into a single transaction

	tx, err := repo.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 
	}

	defer func() {
		// commit or roll back based on final error value.

		if err != nil {
			_ = tx.Rollback(ctx)
			return
		}

		err = tx.Commit(ctx)
	}()


	var exists bool
	err = tx.QueryRow(
		ctx,
		`SELECT EXISTS(
         SELECT 1 FROM registrations
         WHERE event_id = $1 AND email = $2
     )`,
		req.EventID,
		req.Email,
	).Scan(&exists)
	if err != nil {
		return 
	}
	if exists {
		err = registration.ErrAlreadyRegistered
		return 
	}

	// 2) Lock event row and check capacity

	var capacity int
	var current int
	err = tx.QueryRow(
		ctx,
		`
		SELECT e.capacity, 
			(SELECT COUNT(*) FROM registrations r WHERE r.event_id = e.id) AS current
		FROM events e
		WHERE e.id = $1
		FOR UPDATE
		`,
		req.EventID,
	).Scan(&capacity, &current)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = event.ErrNotFound
			return 
		}

		return
	}

	// 2) Enforce capacity

	if current >= capacity {
		err = registration.ErrEventFull
		return 
	}

	// Build registration from DTO
	reg = registration.NewFromCreateRequest(req)

	// Insert registration (still insider the transaction)

	_, err = tx.Exec(ctx,
		`INSERT INTO registrations (id,event_id,name,email, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6)
		`,
		reg.ID, reg.EventID, reg.Name, reg.Email, reg.CreatedAt, reg.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "registrations_event_email_uniq" {
			err = registration.ErrAlreadyRegistered
			return

		}
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


func (repo *RegistrationRepo)ListByEvent(ctx context.Context,eventID string ) (regs []registration.Registration,err error){
	rows, err := repo.pool.Query(ctx,
	`
	SELECT id, event_id,name, email, created_at,updated_at
	FROM registrations
	WHERE event_id = $1
	ORDER BY created_at ASC, id ASC
	`,
	eventID,
	
	)

	if err != nil {
		return
	}

	defer rows.Close()

	regs = make([]registration.Registration,0)

	for rows.Next(){
		var r registration.Registration

		err = rows.Scan(&r.ID, &r.EventID, &r.Name, &r.Email,&r.CreatedAt,&r.UpdatedAt)

		if err != nil {
			return
		}
		regs = append(regs, r)
	}

	err = rows.Err()

	if err != nil {
		return 
	}

	// in the event i want a 404 if the event itself does not exist

	if len(regs) == 0 {
		// check if event exists at all
		var dummy string

		err = repo.pool.QueryRow(ctx,`SELECT id FROM events WHERE id = $1`,eventID).Scan(&dummy)
		if errors.Is(err,pgx.ErrNoRows) {
			err = event.ErrNotFound

			return 
		}

		if err != nil {
			return
		}
	}

	return 
}

// Delete removes a single registration for an event

func(repo *RegistrationRepo)Delete(ctx context.Context, eventID,registrationID string) (err error) {
	tag, err := repo.pool.Exec(ctx, `DELETE FROM registrations WHERE id = $1 AND event_id = $2`,registrationID, eventID)

	if err != nil {
		return
	}

	if tag.RowsAffected() == 0 {
		err = registration.ErrNotFound

		return 
	}

	return
}


