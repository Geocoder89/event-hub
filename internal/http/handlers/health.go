package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	ping func() error
}

// create a new instance of the health handler
func NewHealthHandler(ping func() error) *HealthHandler {
	return &HealthHandler{
		ping: ping,
	}
}

func (h *HealthHandler) Healthz(ctx *gin.Context) {
	ctx.JSON(200, gin.H{"status": "ok"})
}

func (h *HealthHandler) Readyz(ctx *gin.Context) {
	// DB connection check
	if h.ping != nil {
		err := h.ping()

		if err != nil {
			RespondError(ctx, http.StatusServiceUnavailable, "not_ready", "not_available", gin.H{"dependency": "postgres"})
			return
		}

	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ready"})
}
