package worker

import "testing"

func TestTraceCarrierFromPayload_ExtractsFields(t *testing.T) {
	payload := []byte(`{
		"requestId":"req-1",
		"eventId":"evt-2",
		"registrationId":"reg-3",
		"requestedBy":"usr-4",
		"userId":"usr-5"
	}`)

	carrier, err := traceCarrierFromPayload(payload)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}

	if carrier.RequestID != "req-1" {
		t.Fatalf("expected requestId req-1, got %q", carrier.RequestID)
	}
	if carrier.EventID != "evt-2" {
		t.Fatalf("expected eventId evt-2, got %q", carrier.EventID)
	}
	if carrier.RegistrationID != "reg-3" {
		t.Fatalf("expected registrationId reg-3, got %q", carrier.RegistrationID)
	}
	if carrier.RequestedBy != "usr-4" {
		t.Fatalf("expected requestedBy usr-4, got %q", carrier.RequestedBy)
	}
	if carrier.UserID != "usr-5" {
		t.Fatalf("expected userId usr-5, got %q", carrier.UserID)
	}
}

func TestRequestIDFromPayload_InvalidJSONReturnsEmpty(t *testing.T) {
	got := requestIDFromPayload([]byte("{not-json"))
	if got != "" {
		t.Fatalf("expected empty requestId, got %q", got)
	}
}
