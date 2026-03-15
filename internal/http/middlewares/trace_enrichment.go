package middlewares

import (
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TraceEnrichment adds correlation identifiers to the active request span.
// This keeps API traces queryable by request/job/user/event IDs.
func TraceEnrichment() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		span := trace.SpanFromContext(c.Request.Context())
		if !span.SpanContext().IsValid() {
			return
		}

		attrs := make([]attribute.KeyValue, 0, 4)

		if reqID, ok := getContextString(c, CtxRequestID); ok && reqID != "" {
			attrs = append(attrs, attribute.String("request.id", reqID))
		}

		if userID, ok := getContextString(c, CtxUserID); ok && userID != "" {
			attrs = append(attrs, attribute.String("user.id", userID))
		}

		if jobID, ok := getContextString(c, CtxJobID); ok && jobID != "" {
			attrs = append(attrs, attribute.String("job.id", jobID))
		}

		if eventID := eventIDFromRoute(c); eventID != "" {
			attrs = append(attrs, attribute.String("event.id", eventID))
		}

		if len(attrs) == 0 {
			return
		}

		span.SetAttributes(attrs...)
	}
}

func eventIDFromRoute(c *gin.Context) string {
	route := c.FullPath()
	if route == "" || !strings.Contains(route, "/events/:id") {
		return ""
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return ""
	}

	return id
}
