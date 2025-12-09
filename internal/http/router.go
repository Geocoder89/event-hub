package http

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/geocoder89/eventhub/internal/http/handlers"
	"github.com/geocoder89/eventhub/internal/repo/memory"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)


func NewRouter(log *slog.Logger, pool *pgxpool.Pool) *gin.Engine {
	cfgEnv := os.Getenv("APP_ENV")

	if cfgEnv != "dev" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()

	// middleware

	r.Use(gin.Recovery())
	r.Use(RequestID())
	r.Use(RequestLogger(log))

	// health 
	ping := func() error {
		if pool == nil {
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
		defer cancel()

		return pool.Ping(ctx)
	}

	// Routes 
	h := handlers.NewHealthHandler(ping)
	r.GET("/healthz", h.Healthz)
	r.GET("/readyz",h.Readyz)

	// events stored in memory for now

	eventsRepo := memory.NewEventsRepo()
	eventsHandler := handlers.NewEventsHandler(eventsRepo)
	r.POST("/events", eventsHandler.CreateEvent)
	r.GET("/events",eventsHandler.ListEvents)
	r.GET("/events/:id",eventsHandler.GetEventById)

	return r
}
