package event

import "time"



type Event struct {
	ID string `json:"id"`
	Title string `json:"title"`
	Description string `json:"description,omitempty"`
	City string `json:"city,omitempty"`
	StartAt time.Time `json:"startAt"`
	Capacity int	`json:"capacity"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type CreateEventRequest struct {
	Title string `json:"title" binding:"required,min=2"` // required and minimum must be 2
	Description string `json:"description"`
	City string `json:"city"`
	StartAt time.Time `json:"startAt" binding:"required"` // required
	Capacity int `json:"capacity" binding:"required,gt=0"` // required and must be greater than 0
}