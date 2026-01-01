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

func NewRouter(log *slog.Logger, pool *pgxpool.Pool, cfg config.Config) *gin.Engine {
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
	refreshTokensRepo := postgres.NewRefreshTokensRepo(pool)

	// JWT Manager
	jwtManager := auth.NewManager(
		cfg.JWTSecret,
		time.Duration(cfg.JWTAccessTTLMinutes)*time.Minute, // 60mins
		time.Duration(cfg.JWTRefreshTTLDays)*24*time.Hour,
	)
	// Wire up more handler
	eventsHandler := handlers.NewEventsHandler(eventsRepo)
	registrationHandler := handlers.NewRegistrationHandler(registrationRepo)
	authHandler := handlers.NewAuthHandler(usersRepo, usersRepo, jwtManager, refreshTokensRepo, cfg)
	authMiddleware := middlewares.NewAuthMiddleware(jwtManager)

	// rate limiter middleware

	loginLimiter := middlewares.NewRateLimiter(5, 1*time.Minute)
	signupLimiter := middlewares.NewRateLimiter(3, 1*time.Minute)
	refreshLimiter := middlewares.NewRateLimiter(10, 1*time.Minute)
	registerLimiter := middlewares.NewRateLimiter(5, 1*time.Minute)

	// public routes
	r.GET("/healthz", h.Healthz)
	r.GET("/readyz", h.Readyz)

	r.POST("/signup", signupLimiter.RateLimiterMiddleware(middlewares.KeyByIP), authHandler.SignUp)
	r.POST("/login", loginLimiter.RateLimiterMiddleware(middlewares.KeyByIP), authHandler.Login)
	r.POST("/auth/refresh", refreshLimiter.RateLimiterMiddleware(middlewares.KeyByIP), authHandler.Refresh)
	r.POST("/auth/logout", authHandler.Logout)

	// public events browsing.
	r.GET("/events", eventsHandler.ListEvents)
	r.GET("/events/:id", eventsHandler.GetEventById)

	// authenticated routes only authenticated users, can access this route.

	authed := r.Group("/")

	authed.Use(authMiddleware.RequireAuth())

	{
		authed.POST("/events/:id/register", registerLimiter.RateLimiterMiddleware(middlewares.KeyByUserOrIP), registrationHandler.Register)
		authed.GET("/events/:id/registrations", registrationHandler.ListForEvent)
		authed.DELETE("/events/:id/registrations/:registrationId", registrationHandler.Cancel)
	}

	// admin authorized route set up.

	admin := authed.Group("/")
	admin.Use(authMiddleware.RequireRole("admin"))

	{
		admin.POST("/events", eventsHandler.CreateEvent)
		admin.PUT("/events/:id", eventsHandler.UpdateEvent)
		admin.DELETE("/events/:id", eventsHandler.DeleteEvent)
		// event registration route
	}

	return r
}
