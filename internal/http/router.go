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
	"github.com/geocoder89/eventhub/internal/queue/redisclient"

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

	redis := redisclient.New(redisclient.Config{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	r := gin.New()

	// middleware

	r.Use(gin.Recovery())
	r.Use(middlewares.RequestID())
	r.Use(middlewares.RequestLogger(log))
	r.Use(middlewares.CORSMiddleware([]string{
		"http://localhost:3000",
	}))
	r.Use(middlewares.SecurityHeaders())
	r.Use(middlewares.MaxBodyBytes(1 << 20)) //1MB max body
	r.Use(middlewares.RequireJSON())         // Require JSON content type for post and put requests.

	readyCheck := func() error {
		// postgres ping
		if pool != nil {

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			err := pool.Ping(ctx)

			if err != nil {
				return err
			}
		}

		// Redis ping

		{
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			err := redis.Ping(ctx)

			if err != nil {
				return err
			}
		}

		return nil
	}

	// health
	h := handlers.NewHealthHandler(readyCheck)

	// events stored in memory for now

	// eventsRepo := memory.NewEventsRepo()
	// change to postgres

	// wire up repositories
	eventsRepo := postgres.NewEventsRepo(pool)
	registrationRepo := postgres.NewRegistrationsRepo(pool)
	usersRepo := postgres.NewUsersRepo(pool)
	refreshTokensRepo := postgres.NewRefreshTokensRepo(pool)
	jobsRepo := postgres.NewJobsRepo(pool)

	// JWT Manager
	jwtManager := auth.NewManager(
		cfg.JWTSecret,
		time.Duration(cfg.JWTAccessTTLMinutes)*time.Minute, // 60mins
		time.Duration(cfg.JWTRefreshTTLDays)*24*time.Hour,
	)
	// Wire up more handler
	eventsHandler := handlers.NewEventsHandler(eventsRepo)
	registrationHandler := handlers.NewRegistrationHandler(registrationRepo)
	jobsHandler := handlers.NewJobsHandler(jobsRepo)
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
		authed.POST("/events/:id/publish", jobsHandler.PublishEvent)
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
