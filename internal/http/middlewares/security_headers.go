package middlewares

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	defaultCSP = "default-src 'none'"
	// Swagger UI page needs CDN assets + inline bootstrap script/style.
	swaggerCSP = "default-src 'self'; base-uri 'none'; frame-ancestors 'none'; object-src 'none'; connect-src 'self'; img-src 'self' data: https:; font-src 'self' https://unpkg.com data:; style-src 'self' 'unsafe-inline' https://unpkg.com; script-src 'self' 'unsafe-inline' https://unpkg.com"
)

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-XSS-Protection", "0")
		if strings.HasPrefix(c.Request.URL.Path, "/swagger") {
			c.Header("Content-Security-Policy", swaggerCSP)
		} else {
			c.Header("Content-Security-Policy", defaultCSP)
		}
		c.Next()
	}
}
