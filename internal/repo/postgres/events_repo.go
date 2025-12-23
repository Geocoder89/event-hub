package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/geocoder89/eventhub/internal/domain/event"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventsRepo struct {
	pool *pgxpool.Pool
}

// constructor function

func NewEventsRepo(pool *pgxpool.Pool) *EventsRepo {
	return &EventsRepo{
		pool: pool,
	}
}

func (r *EventsRepo) Create(ctx context.Context,req event.CreateEventRequest) (event.Event, error) {
	e := event.NewFromCreateRequest(req)

	_, err := r.pool.Exec(ctx,
		`INSERT INTO events(id,title, description, city, start_at, capacity,created_at, updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, e.ID, e.Title, e.Description, e.City, e.StartAt, e.Capacity, e.CreatedAt, e.UpdatedAt)

	if err != nil {
		return event.Event{}, err
	}

	return e, nil

}

func (r *EventsRepo) List(ctx context.Context, filteredEvents event.ListEventsFilter) ([]event.Event, int, error) {
	baseQuery :=
		`SELECT id, 
		title, 
		description,
		city,
		start_at, 
		capacity,
	  created_at,
		updated_at,
		COUNT(*) OVER() AS TOTAL
	FROM events 
	`

	var conds []string
	var args []interface{}

	argsPosition := 1

	// filtered conditional checks.
	if filteredEvents.City != nil {
		conds = append(conds, fmt.Sprintf("city = $%d", argsPosition))
		args = append(args, *filteredEvents.City)
		argsPosition++
	}

	// From filter

	if filteredEvents.From != nil {
		conds = append(conds, fmt.Sprintf("start_at >= $%d", argsPosition))
		args = append(args, *filteredEvents.From)
		argsPosition++
	}

	// to filter
	if filteredEvents.To != nil {
		conds = append(conds, fmt.Sprintf("start_at <= $%d", argsPosition))
		args = append(args, *filteredEvents.To)
		argsPosition++
	}

	query := baseQuery

	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}

	// stable ordering for pagination
	query += fmt.Sprintf(" ORDER BY start_at ASC, id ASC LIMIT $%d OFFSET $%d", argsPosition, argsPosition+1)

	args = append(args, filteredEvents.Limit, filteredEvents.Offset)

	rows, err := r.pool.Query(ctx, query, args...)

	if err != nil {
		return nil, 0, err
	}

	defer rows.Close()

	output := make([]event.Event, 0, filteredEvents.Limit)
	total := 0

	for rows.Next() {
		var e event.Event
		var t int

		err = rows.Scan(&e.ID, &e.Title, &e.Description, &e.City, &e.StartAt, &e.Capacity, &e.CreatedAt, &e.UpdatedAt, &t)

		if err != nil {
			return nil, 0, err
		}

		total = t
		output = append(output, e)
	}

	err = rows.Err()

	if err != nil {
		return nil, 0, err
	}

	return output, total, nil
}

func (r *EventsRepo) GetByID(ctx context.Context,id string) (event.Event, error) {
	var e event.Event
	err := r.pool.QueryRow(ctx, `SELECT id, title, description,city,start_at,capacity,created_at,updated_at FROM events WHERE id =$1`, id).Scan(&e.ID, &e.Title, &e.Description, &e.City, &e.StartAt, &e.Capacity, &e.CreatedAt, &e.UpdatedAt)

	if err != nil {
		return event.Event{}, event.ErrNotFound
	}

	return e, nil
}

func (r *EventsRepo) Update(ctx context.Context,id string, req event.UpdateEventRequest) (event.Event, error) {
	var e event.Event

	err := r.pool.QueryRow(
		ctx,
		`UPDATE events
			SET title = $2,
					description = $3,
					city = $4,
					start_at = $5,
					capacity = $6,
					updated_at = NOW()
		WHERE id = $1
		RETURNING id, title, description, city, start_at, capacity,created_at,updated_at`,
		id,
		req.Title,
		req.Description,
		req.City,
		req.StartAt,
		req.Capacity,
	).Scan(
		&e.ID,
		&e.Title,
		&e.Description,
		&e.City,
		&e.StartAt,
		&e.Capacity,
		&e.CreatedAt,
		&e.UpdatedAt,
	)

	if err != nil {
		// if there are no rows matching the id
		if errors.Is(err, pgx.ErrNoRows) {
			return event.Event{}, event.ErrNotFound
		}
		// if it is any other type of error
		return event.Event{}, err
	}

	return e, nil
}

func (r *EventsRepo) Delete(ctx context.Context,id string) error {
	query, err := r.pool.Exec(ctx, `
		DELETE from events WHERE id = $1
	`, id)

	if err != nil {

		return err
	}

	// if no rows were deleted as a result return a not found error
	if query.RowsAffected() == 0 {
		return event.ErrNotFound
	}

	return nil
}
