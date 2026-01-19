package jobs

import (
	"encoding/json"
	"time"
)

const TypeRegistrationConfirmation = "registration.confirmation"

type RegistrationConfirmationPayload struct {
	RegistrationID string    `json:"registrationId"`
	EventID        string    `json:"eventId"`
	Email          string    `json:"email"`
	Name           string    `json:"name"`
	RequestedAt    time.Time `json:"requestedAt"`
}

func (p RegistrationConfirmationPayload) JSON() (json.RawMessage, error) {
	b, err := json.Marshal(p)

	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
