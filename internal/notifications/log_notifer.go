package notifications

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

type LogNotifier struct{}

func NewLogNotifier() *LogNotifier { return &LogNotifier{} }

func (n *LogNotifier) SendRegistrationConfirmation(ctx context.Context, in SendRegistrationConfirmationInput) error {
	// Optional: simulate slow provider
	if msStr := os.Getenv("NOTIFIER_SLEEP_MS"); msStr != "" {
		ms, _ := strconv.Atoi(msStr)
		if ms > 0 {
			select {
			case <-time.After(time.Duration(ms) * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// Optional: simulate provider outage
	if os.Getenv("NOTIFIER_FAIL") == "1" {
		return fmt.Errorf("provider down (simulated)")
	}

	log.Printf("notification.registration_confirmation email=%s name=%s event=%s registration=%s",
		in.Email, in.Name, in.EventID, in.RegistrationID,
	)
	return nil
}
