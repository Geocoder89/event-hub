package jobs

import (
	"encoding/json"
	"time"
)

const (
	TypeEventPublish = "event.publish"
)

type EventPublishPayload struct {
	EventID     string    `json:"eventId"`
	RequestedBy string    `json:"requestedBy"`
	RequestedAt time.Time `json:"requestedAt"`
	RequestID   string    `json:"requestId,omitempty"`
}

// Helper to convert payload to json.RawMessage

func (p EventPublishPayload) ToJSONRaw() (json.RawMessage, error) {
	b, err := json.Marshal(p)

	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
