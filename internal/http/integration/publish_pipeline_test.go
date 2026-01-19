package integration__test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/geocoder89/eventhub/internal/auth"
	"github.com/geocoder89/eventhub/internal/config"
	apphttp "github.com/geocoder89/eventhub/internal/http"
	"github.com/geocoder89/eventhub/internal/queue/worker"
	"github.com/geocoder89/eventhub/internal/repo/postgres"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupPipelineTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool, config.Config) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		dsn = "postgres://eventhub:eventhub@127.0.0.1:5433/eventhub?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pg pool: %v", err)
	}

	cfg := config.Config{
		Env:                 "test",
		DBURL:               dsn,
		JWTSecret:           "test-secret",
		JWTAccessTTLMinutes: 60,
		JWTRefreshTTLDays:   7,
		RedisAddr:           "127.0.0.1:6379",
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	router := apphttp.NewRouter(logger, pool, cfg)

	return router, pool, cfg

}

func resetPipelineDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		TRUNCATE refresh_tokens, registrations, jobs, events, users RESTART IDENTITY CASCADE
	`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func TestPublishPipeline_EndToEnd(t *testing.T) {
	router, pool, cfg := setupPipelineTestRouter(t)
	resetPipelineDB(t, pool)
	defer resetPipelineDB(t, pool)

	// Seed data

	eventID := seedEvent(t, pool, 2)

	jwtManager := auth.NewManager(cfg.JWTSecret, 60*time.Minute, 7*24*time.Hour)
	adminID := uuid.NewString()
	token, err := jwtManager.GenerateAccessToken(adminID, "admin@example.com", "admin")

	if err != nil {
		t.Fatalf("token: %v", err)
	}

	// Call publish endpoint
	req := httptest.NewRequest(http.MethodPost, "/admin/events/"+eventID+"/publish", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("publish got %d body=%s", w.Code, w.Body.String())
	}

	// Process job with worker step
	jobsRepo := postgres.NewJobsRepo(pool)
	eventsRepo := postgres.NewEventsRepo(pool)

	wk := worker.New(worker.Config{
		PollInterval:  10 * time.Millisecond,
		WorkerID:      "test-worker",
		Concurrency:   1,
		ShutdownGrace: 1 * time.Second,
	}, jobsRepo, eventsRepo)

	processed, err := wk.ProcessOne(context.Background())

	if err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}

	if !processed {
		t.Fatalf("expected a job to be processed")
	}

	// Assert side effect: published_at is set
	var publishedAt *time.Time
	err = pool.QueryRow(context.Background(), `SELECT published_at FROM events WHERE id=$1`, eventID).Scan(&publishedAt)
	if err != nil {
		t.Fatalf("select event: %v", err)
	}
	if publishedAt == nil {
		t.Fatalf("expected published_at to be set")
	}
}
