package memory

import (
	"sync"
	"time"

	"github.com/geocoder89/eventhub/internal/domain/event"
	"github.com/google/uuid"
)

type EventsRepo struct {
	mu    sync.RWMutex
	items map[string]event.Event // {"key": "value"}

}

func NewEventsRepo() *EventsRepo {
	return &EventsRepo{
		items: make(map[string]event.Event),
	}
}


func (r *EventsRepo) Create(req event.CreateEventRequest)(event.Event,error) {
	now := time.Now()
	e := event.Event {
		ID: uuid.NewString(),
		Title: req.Title,
		Description: req.Description,
		City: req.City,
		StartAt: req.StartAt,
		Capacity: req.Capacity,
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.mu.Lock()
	r.items[e.ID] = e
	r.mu.Unlock()

	return e, nil
}
