package postgres

import (
	"context"

	"github.com/geocoder89/eventhub/internal/domain/event"
	"github.com/jackc/pgx/v5/pgxpool"
)



type EventsRepo struct {
	pool *pgxpool.Pool
}


// constructor function

func NewEventsRepo(pool *pgxpool.Pool) *EventsRepo{
	return &EventsRepo{
		pool: pool,
	}
}

func(r *EventsRepo) Create(req event.CreateEventRequest) (event.Event, error) {
	e := event.NewFromCreateRequest(req)

	_, err := r.pool.Exec(context.Background(),
	`INSERT INTO events(id,title, description, city, start_at, capacity,created_at, updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, e.ID, e.Title,e.Description,e.City,e.StartAt,e.Capacity,e.CreatedAt, e.UpdatedAt)


	if err != nil {
		return event.Event{}, err
	}

	return e, nil

}

func (r *EventsRepo) List()([]event.Event, error) {
	rows, err := r.pool.Query(context.Background(), `SELECT id, title, description, city, start_at, capacity, created_at,updated_at FROM events 
	ORDER BY start_at ASC`)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	output := make([]event.Event, 0)


	for rows.Next() {
		var e event.Event

		err = rows.Scan(&e.ID, &e.Title, &e.Description, &e.City, &e.StartAt, &e.Capacity,&e.CreatedAt, &e.UpdatedAt)

		if err != nil {
			return nil, err
		}

		output = append(output, e)
	}

	return output, rows.Err()
}


func(r *EventsRepo)GetByID(id string)(event.Event, error){
	var e event.Event
	err := r.pool.QueryRow(context.Background(), `SELECT id, title, description,city,start_at,capacity,created_at,updated_at FROM events WHERE id =$1`,id).Scan(&e.ID,&e.Title,&e.Description,&e.City,&e.StartAt,&e.Capacity,&e.CreatedAt,&e.UpdatedAt)

	if err != nil {
		return event.Event{},event.ErrNotFound
	}

	return e,nil
}