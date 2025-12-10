package event

import (
	"time"

	"github.com/google/uuid"
)



func NewFromCreateRequest (req CreateEventRequest) Event {
	now := time.Now()

	return Event{
		ID: uuid.NewString(),
		Title: req.Title,
		Description: req.Description,
		City: req.City,
		StartAt: req.StartAt,
		Capacity: req.Capacity,
		CreatedAt: now,
		UpdatedAt: now,
	}
}