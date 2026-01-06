package jobs

// PublishEventPayload describes the data needed to publish an event asynchronously.
// Keep payload minimal and ID-based; worker will load details from DB.
type PublishEventPayload struct {
	EventID  string `json:"eventId"`
	ActorID  string `json:"actorId,omitempty"`  // optional: user/admin who initiated
	RequestID string `json:"requestId,omitempty"` // optional: correlation
}

// SendRegistrationConfirmationPayload is used to send a confirmation email/message.
type SendRegistrationConfirmationPayload struct {
	RegistrationID string `json:"registrationId"`
	UserID         string `json:"userId"`
	EventID         string `json:"eventId"`
}

// ExportRegistrationsCSVPayload generates a CSV export for an event.
type ExportRegistrationsCSVPayload struct {
	EventID  string `json:"eventId"`
	ActorID  string `json:"actorId,omitempty"`
}
