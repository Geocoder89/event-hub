package http

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/geocoder89/eventhub/internal/auth"
	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/http/handlers"
	"github.com/geocoder89/eventhub/internal/http/middlewares"

	// "github.com/geocoder89/eventhub/internal/repo/memory"
	"github.com/geocoder89/eventhub/internal/repo/postgres"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(log *slog.Logger, pool *pgxpool.Pool,cfg config.Config) *gin.Engine {
	cfgEnv := os.Getenv("APP_ENV")

	if cfgEnv != "dev" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()

	// middleware

	r.Use(gin.Recovery())
	r.Use(middlewares.RequestID())
	r.Use(middlewares.RequestLogger(log))

	
	ping := func() error {
		if pool == nil {
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		return pool.Ping(ctx)
	}

// health
	h := handlers.NewHealthHandler(ping)
	
	// events stored in memory for now

	// eventsRepo := memory.NewEventsRepo()
	// change to postgres

	// wire up repositories
	eventsRepo := postgres.NewEventsRepo(pool)
	registrationRepo := postgres.NewRegistrationsRepo(pool)
	usersRepo := postgres.NewUsersRepo(pool)

	// JWT Manager
	jwtManager := auth.NewManager(
		cfg.JWTSecret,
		time.Duration(cfg.JWTAccessTTLMinutes)*time.Minute, // 60mins
	)
	// Wire up more handler
	eventsHandler := handlers.NewEventsHandler(eventsRepo)
	registrationHandler := handlers.NewRegistrationHandler(registrationRepo)
	authHandler := handlers.NewAuthHandler(usersRepo,jwtManager)
	authMiddleware := middlewares.NewAuthMiddleware(jwtManager)


	// public routes
	r.GET("/healthz", h.Healthz)
	r.GET("/readyz", h.Readyz)
	r.POST("/login",authHandler.Login)
	r.GET("/events", eventsHandler.ListEvents)
	r.GET("/events/:id", eventsHandler.GetEventById)
	r.POST("/events/:id/register", registrationHandler.Register)

	// protected routes

	secured := r.Group("/")
	secured.Use(authMiddleware.RequireAuth())

	{
		secured.POST("/events", eventsHandler.CreateEvent)
	secured.PUT("/events/:id", eventsHandler.UpdateEvent)
	secured.DELETE("/events/:id", eventsHandler.DeleteEvent)
	// event registration route
	secured.GET("/events/:id/registrations", registrationHandler.ListForEvent)
	secured.DELETE("/events/:id/registrations/:registrationId", registrationHandler.Cancel)
	}
	
	return r
}
