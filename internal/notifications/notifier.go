package notifications

import "context"



type SendRegistrationConfirmationInput struct {
	Email string
	Name string
	EventID string
	RegistrationID string
}

type Notifier interface {
	SendRegistrationConfirmation(ctx context.Context, input SendRegistrationConfirmationInput) error
}