package worker

import (
	cryptorand "crypto/rand"
	"math"
	"math/big"
	"time"
)

func ExponentialBackoff(attempt int) time.Duration {
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
	delay += jitterDuration(250 * time.Millisecond)
	return delay
}

func jitterDuration(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}

	maxJitter := int64(max / time.Millisecond)
	if maxJitter <= 0 {
		return 0
	}

	n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(maxJitter))
	if err != nil {
		return 0
	}

	return time.Duration(n.Int64()) * time.Millisecond
}
