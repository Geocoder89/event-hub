package worker

import (
	"math"
	"math/rand"
	"time"
)



func ExponentialBackoff (attempt int) time.Duration {
	base := 2 * time.Second

	capDelay := 5 * time.Minute
	// attempt=0 => 2s
	// attempt=1 => 4s
	// attempt=2 => 8s

	multiple := math.Pow(2, float64(attempt))
	delay := time.Duration(float64(base) * multiple)

	if delay > capDelay {
		delay = capDelay
	}

	// small jitter (0–250ms) to avoid thundering herd
	delay += time.Duration(rand.Intn(250)) * time.Millisecond
	return delay
}