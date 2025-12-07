package handlers

import "github.com/gin-gonic/gin"


type HealthHandler struct {}


// create a new instance of the health handler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}


func (h *HealthHandler) Healthz(ctx *gin.Context) {
	ctx.JSON(200, gin.H{"status": "ok"})
}

func (h *HealthHandler) Readyz(ctx *gin.Context) {
	// Day 1 phase : always ready, we look at deeper things like DB connection

	ctx.JSON(200, gin.H{"status": "ready"})
}