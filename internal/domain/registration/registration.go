package registration

import (
	"errors"
	"github.com/google/uuid"
	"time"
)

type Registration struct {
	ID        string    `json:"id"`
	EventID   string    `json:"eventId"`
	UserID    string    `json:"userId"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// if you are already registered.
var ErrAlreadyRegistered = errors.New("registration already exists")

// error if event is full
var ErrEventFull = errors.New("event is full")
var ErrNotFound = errors.New("registration not found")

type CreateRegistrationRequest struct {
	EventID string `json:"-"`
	UserID  string `json:"-"`
	Name    string `json:"name" binding:"required,min=2"`
	Email   string `json:"email" binding:"required,email"`
}

// A factory to build a Registration from the incoming DTO

func NewFromCreateRequest(req CreateRegistrationRequest) Registration {
	now := time.Now()
	return Registration{
		ID:        uuid.NewString(),
		EventID:   req.EventID,
		UserID:    req.UserID, // added user id from access token into request field.
		Name:      req.Name,
		Email:     req.Email,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
