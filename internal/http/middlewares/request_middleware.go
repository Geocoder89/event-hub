package middlewares

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-Id"

func RequestID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Get the request header
		id := ctx.GetHeader(requestIDHeader)

		// if there
		if id == "" {
			id = uuid.NewString()
		}
		//
		ctx.Writer.Header().Set(requestIDHeader, id)

		ctx.Set(CtxRequestID, id)
		ctx.Set("request_id", id) // legacy compatibility for older handlers/helpers

		ctx.Next()

	}
}

func RequestLogger() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()

		route := ctx.FullPath()
		if route == "" {
			route = ctx.Request.URL.Path // fallback (e.g. 404)
		}

		method := ctx.Request.Method

		ctx.Next()

		lat := time.Since(start)
		status := ctx.Writer.Status()

		reqID, _ := ctx.Get(CtxRequestID)

		logAttrs := []any{
			"method", method,
			"route", route,
			"status", status,
			"latency_ms", lat.Milliseconds(),
			"request_id", reqID,
		}

		if jobID, ok := ctx.Get(CtxJobID); ok {
			if jobIDStr, ok := jobID.(string); ok && jobIDStr != "" {
				logAttrs = append(logAttrs, "job_id", jobIDStr)
			}
		}

		slog.Default().InfoContext(ctx.Request.Context(), "http_request", logAttrs...)
	}
}
