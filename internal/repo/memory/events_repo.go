package memory

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/geocoder89/eventhub/internal/domain/event"
	"github.com/google/uuid"
)

// errors not found

var ErrNotFound = errors.New("Event not found")

type EventsRepo struct {
	mu    sync.RWMutex
	items map[string]event.Event // {"key": "value"}

}

func NewEventsRepo() *EventsRepo {
	return &EventsRepo{
		items: make(map[string]event.Event),
	}
}

func (r *EventsRepo) Create(req event.CreateEventRequest) (event.Event, error) {
	now := time.Now()
	e := event.Event{
		ID:          uuid.NewString(),
		Title:       req.Title,
		Description: req.Description,
		City:        req.City,
		StartAt:     req.StartAt,
		Capacity:    req.Capacity,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	r.mu.Lock()
	r.items[e.ID] = e
	r.mu.Unlock()

	return e, nil
}

func (r *EventsRepo) GetByID(id string) (event.Event, error) {
	r.mu.RLock()
	e, ok := r.items[id]
	r.mu.RUnlock()

	if !ok {
		return event.Event{}, ErrNotFound
	}
	return e, nil
}

func (r *EventsRepo) List() ([]event.Event, error) {
	// Rlock to make safe reads,so multiple reads can happen concurrently without blocking each other.
	r.mu.RLock()
	// Create an out slice using the make function which is of size 0 and has a capacity of the length of items
	out := make([]event.Event, 0, len(r.items))
	for _, e := range r.items {
		out = append(out, e)
	}
	// a good improvement is to handle stable ordering. more or less ordering by startAt

	sort.Slice(out, func(i, j int) bool {
		return out[i].StartAt.Before(out[j].StartAt)
	})

	// at this stage we expect a value so we return it

	return out, nil

}
