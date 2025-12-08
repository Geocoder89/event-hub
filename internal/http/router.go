package http

import (
	"log/slog"
	"os"

	"github.com/geocoder89/eventhub/internal/http/handlers"
	"github.com/geocoder89/eventhub/internal/repo/memory"
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

	// events stored in memory for now

	eventsRepo := memory.NewEventsRepo()
	eventsHandler := handlers.NewEventsHandler(eventsRepo)
	r.POST("/events", eventsHandler.CreateEvent)

	return r
}
