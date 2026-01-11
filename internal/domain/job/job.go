package job

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
)

var ErrJobNotFound = errors.New("job not found")

type Job struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	Status      Status          `json:"status"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"maxAttempts"`
	RunAt       time.Time       `json:"runAt"`
	LockedAt    *time.Time      `json:"lockedAt,omitempty"`
	LockedBy    *string         `json:"lockedBy,omitempty"`
	LastError   *string         `json:"lastError,omitempty"`
	// new Idempotency key
	IdempotencyKey *string 			`json:"idempotencyKey,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type CreateRequest struct {
	Type        string
	Payload     json.RawMessage
	RunAt       time.Time
	MaxAttempts int
	IdempotencyKey *string
}

func New(req CreateRequest) Job {
	now := time.Now().UTC()

	maxA := req.MaxAttempts

	if maxA <= 0 {
		maxA = 25
	}

	runAt := req.RunAt

	if runAt.IsZero() {
		runAt = now
	}

	return Job{
		ID:          uuid.NewString(),
		Type:        req.Type,
		Payload:     req.Payload,
		Status:      StatusPending,
		Attempts:    0,
		MaxAttempts: maxA,
		RunAt:       runAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
