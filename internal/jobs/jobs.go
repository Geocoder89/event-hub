package jobs

import (
	"time"

	"github.com/google/uuid"
)

// a Job is the core representation of a unit of asynchronous work.
// this maps to a future jobs table

type Job struct {
	ID        string    `json:"id"`
	Type      JobType   `json:"type"`
	Payload   []byte    `json:"payload"` // raw json
	Status    JobStatus `json:"status"`
	Attempts  int       `json:"attempts"`
	Maxtries  int       `json:"maxTries"`
	RunAt     time.Time `json:"runAt"`
	LastError *string   `json:"lastError,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

//  creation of a new pending job with defaults.

func NewJob(t JobType, payloadJSON []byte, runAt time.Time) (Job, error) {
	if !t.IsValid() {
		return Job{}, ErrInvalidJobType
	}

	now := time.Now().UTC()

	if runAt.IsZero() {
		runAt = now
	}

	j := Job{
		ID:        uuid.NewString(),
		Type:      t,
		Payload:   payloadJSON,
		Status:    JobPending,
		Attempts:  0,
		Maxtries:  5,
		RunAt:     runAt,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return j, nil
}
