package middlewares

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RateLimiter struct {
	mu      sync.Mutex
	window  time.Duration
	limit   int
	clients map[string]*clientBucket
}

type clientBucket struct {
	count     int
	windowEnd time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:   limit,
		window:  window,
		clients: make(map[string]*clientBucket),
	}
}

// Middleware returns a gin.HandlerFunc that enforces rate limit for a derived key

func (rl *RateLimiter) RateLimiterMiddleware(keyFn func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := keyFn(c)

		if key == "" {
			// fallback to IP if key cannot be derived

			key = clientIP(c)
		}

		now := time.Now()

		rl.mu.Lock()

		b, ok := rl.clients[key]

		if !ok || now.After(b.windowEnd) {
			rl.clients[key] = &clientBucket{
				count:     1,
				windowEnd: now.Add(rl.window),
			}

			rl.mu.Unlock()
			c.Next()
			return
		}

		if b.count >= rl.limit {
			retryAfter := int(time.Until(b.windowEnd).Seconds())

			if retryAfter < 0 {
				retryAfter = 0
			}

			rl.mu.Unlock()

			c.Header("Retry-After", itoa(retryAfter))

			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "rate_limited",
					"message": "Too many requests. Please try again shortly.",
				},
			})

			return
		}

		b.count++
		rl.mu.Unlock()
		c.Next()
	}
}

// helper functions

// for unauthenticated endpoints: rate limit by IP
func KeyByIP(c *gin.Context) string {
	return clientIP(c)
}

// For authenticated endpoints: rate limit by userID if available

func KeyByUserOrIP(c *gin.Context) string {
	id, ok := UserIDFromContext(c)

	if ok && id != "" {
		return "user:" + id
	}

	return clientIP(c)
}

func clientIP(c *gin.Context) string {
	// Gin’s ClientIP respects X-Forwarded-For / X-Real-IP if configured.
	ip := c.ClientIP()

	// Normalize ipv6 zone in a defensive manner

	host, _, err := net.SplitHostPort(ip)

	if err == nil && host != "" {
		return host
	}

	return ip
}

// tiny int->string helper.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [32]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return strings.TrimSpace(string(b[i:]))
}
