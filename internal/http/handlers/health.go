package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	ready func() error
}

// create a new instance of the health handler
func NewHealthHandler(ready func() error) *HealthHandler {
	return &HealthHandler{
		ready: ready,
	}
}

func (h *HealthHandler) Healthz(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"ok":true})
}

func (h *HealthHandler) Readyz(ctx *gin.Context) {

	if h.ready != nil {
		err := h.ready()

		if err != nil {
			RespondError(ctx, http.StatusServiceUnavailable, "not_ready", "not_available", gin.H{"ok": false, "error": err.Error(),})
			return
		}

	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ready"})
}
