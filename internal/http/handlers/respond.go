package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIError struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	RequestID string      `json:"requestId,omitempty"`
	Details   interface{} `json:"details,omitempty"`
}

func requestIDFrom(ctx *gin.Context) string {
	v, ok := ctx.Get("request_id")

	if ok {
		s, ok := v.(string)
		if ok && s != "" {
			return s
		}
	}

	// fallback header
	return ctx.GetHeader("X-Request-Id")
}

func RespondError(ctx *gin.Context, status int, code, message string, details interface{}) {
	ctx.JSON(status, gin.H{
		"error": APIError{
			Code:      code,
			Message:   message,
			RequestID: requestIDFrom(ctx),
			Details:   details,
		},
	})
}

func RespondBadRequest(ctx *gin.Context, message string, details interface{}) {
	RespondError(ctx, http.StatusBadRequest, "invalid_request", message, details)
}

func RespondNotFound(ctx *gin.Context, message string) {
	RespondError(ctx, http.StatusNotFound, "not_found", message, nil)
}

func RespondInternal(ctx *gin.Context, message string) {
	RespondError(ctx, http.StatusInternalServerError, "internal_error", message, nil)
}

func RespondConflict(ctx *gin.Context, code, message string) {
	RespondError(ctx,http.StatusConflict, code,message,nil)
}
