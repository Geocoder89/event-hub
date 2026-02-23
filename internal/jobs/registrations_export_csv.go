package jobs

import (
	"encoding/json"
	"time"
)

const TypeRegistrationsExportCSV = "registrations.export_csv"

type RegistrationsExportCSVPayload struct {
	EventID     string    `json:"eventId"`
	RequestedBy string    `json:"requestedBy"`
	RequestedAt time.Time `json:"requestedAt"`
	RequestID   string    `json:"requestId,omitempty"`
}

func (p RegistrationsExportCSVPayload) JSON() (json.RawMessage, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(b), nil
}
