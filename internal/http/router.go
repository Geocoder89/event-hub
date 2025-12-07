package http

import (
	"log/slog"
	"os"

	"github.com/geocoder89/eventhub/internal/http/handlers"
	"github.com/gin-gonic/gin"
)


func NewRouter(log *slog.Logger) *gin.Engine {
	cfgEnv := os.Getenv("APP_ENV")

	if cfgEnv != "dev" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()

	// middleware

	r.Use(gin.Recovery())
	r.Use(RequestID())
	r.Use(RequestLogger(log))

	// Routes 
	h := handlers.NewHealthHandler()
	r.GET("/healthz", h.Healthz)
	r.GET("/readyz",h.Readyz)

	return r
}
