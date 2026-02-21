package middlewares

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
)

type AdminAuditWriter interface {
	Write(
		ctx context.Context,
		actorUserID, actorEmail, actorRole, action, resourceType, resourceID, requestID string,
		statusCode int,
		details map[string]any,
	) error
}

func AdminAudit(writer AdminAuditWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if writer == nil {
			return
		}

		method := c.Request.Method
		if !isMutatingMethod(method) {
			return
		}

		route := c.FullPath()
		if route == "" || !strings.HasPrefix(route, "/admin/") {
			return
		}

		actorRole, _ := getContextString(c, CtxRole)
		if actorRole == "" {
			actorRole = "admin"
		}

		actorUserID, _ := getContextString(c, CtxUserID)
		actorEmail, _ := getContextString(c, CtxEmail)
		requestID, _ := getContextString(c, CtxRequestID)

		resourceType, resourceID := deriveResource(c, route)

		details := map[string]any{
			"method": method,
			"route":  route,
			"path":   c.Request.URL.Path,
			"query":  c.Request.URL.RawQuery,
		}

		if jobID, ok := getContextString(c, CtxJobID); ok && jobID != "" {
			details["jobId"] = jobID
		}

		if err := writer.Write(
			c.Request.Context(),
			actorUserID,
			actorEmail,
			actorRole,
			method+" "+route,
			resourceType,
			resourceID,
			requestID,
			c.Writer.Status(),
			details,
		); err != nil {
			slog.Default().ErrorContext(c.Request.Context(), "admin.audit_write_failed",
				"request_id", requestID,
				"route", route,
				"method", method,
				"err", err,
			)
		}
	}
}

func isMutatingMethod(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

func deriveResource(c *gin.Context, route string) (string, string) {
	trimmed := strings.TrimPrefix(route, "/admin/")
	segments := strings.Split(trimmed, "/")

	resourceType := "admin"
	if len(segments) > 0 && segments[0] != "" {
		resourceType = segments[0]
	}

	if id := c.Param("id"); id != "" {
		return resourceType, id
	}

	if regID := c.Param("registrationId"); regID != "" {
		return resourceType, regID
	}

	if jobID, ok := getContextString(c, CtxJobID); ok && jobID != "" {
		return resourceType, jobID
	}

	return resourceType, ""
}

func getContextString(c *gin.Context, key ctxKey) (string, bool) {
	v, ok := c.Get(key)
	if !ok {
		return "", false
	}

	s, ok := v.(string)
	if !ok || s == "" {
		return "", false
	}

	return s, true
}
