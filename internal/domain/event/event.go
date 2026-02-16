package event

import (
	"errors"
	"time"
)

type Event struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	City        string    `json:"city,omitempty"`
	StartAt     time.Time `json:"startAt"`
	Capacity    int       `json:"capacity"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// with pointers if optional, it will be nil
type ListEventsFilter struct {
	City   *string
	From   *time.Time
	To     *time.Time
	Query  *string
	Limit  int
	Offset int
}

var ErrNotFound = errors.New("event not found")

type CreateEventRequest struct {
	Title       string    `json:"title" binding:"required,min=3,max=120"`
	Description string    `json:"description" binding:"omitempty,max=1000"`
	City        string    `json:"city" binding:"omitempty,min=2,max=80"`
	StartAt     time.Time `json:"startAt" binding:"required"`
	Capacity    int       `json:"capacity" binding:"required,min=1,max=50000"`
}

// a full update payload, might switch to a patch which optionally provides means for partial updates.
type UpdateEventRequest struct {
	Title       string    `json:"title" binding:"required,min=3,max=120"`
	Description string    `json:"description" binding:"omitempty,max=1000"`
	City        string    `json:"city" binding:"omitempty,min=2,max=80"`
	StartAt     time.Time `json:"startAt" binding:"required"`
	Capacity    int       `json:"capacity" binding:"required,min=1,max=50000"`
}
