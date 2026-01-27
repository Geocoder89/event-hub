package notifications

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("circuit breaker open")

type ProtectedNotifierConfig struct {
	Timeout          time.Duration // hard timeout per send
	FailureThreshold int           // consecutive failures to open circuit
	Cooldown         time.Duration // how long to stay open before half-open
	HalfOpenMaxCalls int           // allow N trial calls in half-open
}

type ProtectedNotifier struct {
	inner Notifier
	cfg   ProtectedNotifierConfig
	mu    sync.Mutex

	state string // "closed" | "open" | "half_open"

	consecutiveFailures int
	openedAt            time.Time
	halfOpenInFlight    int
}

func NewProtectedNotifier(inner Notifier, cfg ProtectedNotifierConfig) *ProtectedNotifier {
	//defaults
	if cfg.Timeout <= 0 {
		cfg.Timeout = 3 * time.Second
	}
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 3
	}
	if cfg.Cooldown <= 0 {
		cfg.Cooldown = 15 * time.Second
	}
	if cfg.HalfOpenMaxCalls <= 0 {
		cfg.HalfOpenMaxCalls = 1
	}

	return &ProtectedNotifier{
		inner: inner,
		cfg:   cfg,
		state: "closed",
	}
}

func (n *ProtectedNotifier) SendRegistrationConfirmation(ctx context.Context, input SendRegistrationConfirmationInput) error {
	// fail-fast gate

	if !n.allowRequest() {
		return ErrCircuitOpen
	}
	// enforce timeout

	sendCtx, cancel := context.WithTimeout(ctx, n.cfg.Timeout)
	defer cancel()

	err := n.inner.SendRegistrationConfirmation(sendCtx, input)

	n.afterRequest(err)

	return err
}

func (n *ProtectedNotifier) allowRequest() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	switch n.state {
	case "closed":
		return true
	case "open":
		// cooldown has passed? move to half open

		if time.Since(n.openedAt) >= n.cfg.Cooldown {
			n.state = "half_open"
			n.halfOpenInFlight = 0
			return true
		}
		return false
	case "half_open":
		if n.halfOpenInFlight >= n.cfg.HalfOpenMaxCalls {
			return false
		}
		n.halfOpenInFlight++
		return true

	default:
		// safe fallback
		return true
	}

}

func (n *ProtectedNotifier) afterRequest(err error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// half-open call just finished
	if n.state == "half_open" && n.halfOpenInFlight > 0 {
		n.halfOpenInFlight--
	}

	if err == nil {
		// success => close circuit and reset counters
		n.consecutiveFailures = 0
		n.state = "closed"
		return
	}

	// failure
	n.consecutiveFailures++

	// if half-open failed, reopen immediately
	if n.state == "half_open" {
		n.state = "open"
		n.openedAt = time.Now()
		return
	}

	// if failures reached threshold, open circuit
	if n.consecutiveFailures >= n.cfg.FailureThreshold {
		n.state = "open"
		n.openedAt = time.Now()
	}
}
