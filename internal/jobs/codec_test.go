
package jobs

import (
	"testing"
	"time"
)

func TestEncodeDecode_PublishEvent(t *testing.T) {
	payload := PublishEventPayload{
		EventID: "event-123",
		ActorID: "user-456",
	}

	b, err := EncodePayload(JobPublishEvent, payload)
	if err != nil {
		t.Fatalf("EncodePayload error: %v", err)
	}

	j, err := NewJob(JobPublishEvent, b, time.Time{})
	if err != nil {
		t.Fatalf("NewJob error: %v", err)
	}

	decoded, err := DecodePayload(j)
	if err != nil {
		t.Fatalf("DecodePayload error: %v", err)
	}

	p, ok := decoded.(PublishEventPayload)
	if !ok {
		t.Fatalf("expected PublishEventPayload, got %T", decoded)
	}

	if p.EventID != payload.EventID {
		t.Fatalf("expected eventId %s, got %s", payload.EventID, p.EventID)
	}
}

func TestEncodePayload_TypeMismatch(t *testing.T) {
	_, err := EncodePayload(JobPublishEvent, SendRegistrationConfirmationPayload{
		RegistrationID: "r1",
		UserID:         "u1",
		EventID:        "e1",
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err != ErrPayloadTypeMismatch {
		t.Fatalf("expected ErrPayloadTypeMismatch, got %v", err)
	}
}

func TestValidatePayload_RequiredIDs(t *testing.T) {
	err := ValidatePayload(JobPublishEvent, PublishEventPayload{EventID: ""})
	if err == nil {
		t.Fatalf("expected error")
	}
}
