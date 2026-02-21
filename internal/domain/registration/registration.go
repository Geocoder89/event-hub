package registration

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"github.com/google/uuid"
	"time"
)

type Registration struct {
	ID           string     `json:"id"`
	EventID      string     `json:"eventId"`
	UserID       string     `json:"userId"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	CheckInToken string     `json:"checkInToken,omitempty"`
	CheckedInAt  *time.Time `json:"checkedInAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// if you are already registered.
var ErrAlreadyRegistered = errors.New("registration already exists")

// error if event is full
var ErrEventFull = errors.New("event is full")
var ErrNotFound = errors.New("registration not found")
var ErrAlreadyCheckedIn = errors.New("registration already checked in")

type CreateRegistrationRequest struct {
	EventID string `json:"-"`
	UserID  string `json:"-"`
	Name    string `json:"name" binding:"required,min=2,max=100"`
	Email   string `json:"email" binding:"required,email,max=254"`
}

// A factory to build a Registration from the incoming DTO

func NewFromCreateRequest(req CreateRegistrationRequest) Registration {
	now := time.Now()
	return Registration{
		ID:           uuid.NewString(),
		EventID:      req.EventID,
		UserID:       req.UserID, // added user id from access token into request field.
		Name:         req.Name,
		Email:        req.Email,
		CheckInToken: newCheckInToken(),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func newCheckInToken() string {
	// 18 random bytes -> 24-char base64url token (no padding), suitable for QR payload.
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return uuid.NewString()
	}

	return base64.RawURLEncoding.EncodeToString(b)
}
