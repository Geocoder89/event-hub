package http

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

		ctx.Set("request_id", id)

		ctx.Next()

	}
}

func RequestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		path := ctx.Request.URL.Path
		method := ctx.Request.Method

		ctx.Next()


		lat := time.Since(start)
		status := ctx.Writer.Status()

	reqID, _ :=	ctx.Get("request_id")

	log.Info("request", "method",method, "path", path, "status", status, "latency_ms", lat.Milliseconds(),"request_id", reqID)
	}
}