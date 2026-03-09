package integration__test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/geocoder89/eventhub/internal/auth"
	"github.com/geocoder89/eventhub/internal/config"
	apphttp "github.com/geocoder89/eventhub/internal/http"
	"github.com/geocoder89/eventhub/internal/notifications"
	"github.com/geocoder89/eventhub/internal/queue/worker"
	"github.com/geocoder89/eventhub/internal/repo/postgres"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupPipelineTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool, config.Config) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := requiredTestDBDSN(t)

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

// func resetPipelineDB(t *testing.T, pool *pgxpool.Pool) {
// 	t.Helper()
// 	_, err := pool.Exec(context.Background(), `
// 		TRUNCATE refresh_tokens, registrations, jobs, events, users RESTART IDENTITY CASCADE
// 	`)
// 	if err != nil {
// 		t.Fatalf("truncate: %v", err)
// 	}
// }

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
	jobsRepo := postgres.NewJobsRepo(pool, nil)
	eventsRepo := postgres.NewEventsRepo(pool, nil)
	deliveriesRepo := postgres.NewNotificationsDeliveriesRepo(pool)
	notifier := notifications.NewLogNotifier()

	wk := worker.New(worker.Config{
		PollInterval:  10 * time.Millisecond,
		WorkerID:      "test-worker",
		Concurrency:   1,
		ShutdownGrace: 1 * time.Second,
	}, jobsRepo, eventsRepo, notifier, deliveriesRepo)

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

func TestPublishPipeline_IdempotentEnqueue(t *testing.T) {
	router, pool, cfg := setupPipelineTestRouter(t)
	resetPipelineDB(t, pool)
	defer resetPipelineDB(t, pool)

	eventID := seedEvent(t, pool, 2)

	jwtManager := auth.NewManager(cfg.JWTSecret, 60*time.Minute, 7*24*time.Hour)
	adminID := uuid.NewString()
	token, err := jwtManager.GenerateAccessToken(adminID, "admin-idempotent@example.com", "admin")
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	callPublish := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/admin/events/"+eventID+"/publish", bytes.NewBufferString(`{}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	w1 := callPublish()
	if w1.Code != http.StatusAccepted {
		t.Fatalf("first publish got %d body=%s", w1.Code, w1.Body.String())
	}

	w2 := callPublish()
	if w2.Code != http.StatusAccepted {
		t.Fatalf("second publish got %d body=%s", w2.Code, w2.Body.String())
	}

	var resp1 struct {
		JobID           string `json:"jobId"`
		Status          string `json:"status"`
		Type            string `json:"type"`
		AlreadyEnqueued bool   `json:"alreadyEnqueued"`
	}
	if err := json.Unmarshal(w1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("decode first publish response: %v body=%s", err, w1.Body.String())
	}

	var resp2 struct {
		JobID           string `json:"jobId"`
		Status          string `json:"status"`
		Type            string `json:"type"`
		AlreadyEnqueued bool   `json:"alreadyEnqueued"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("decode second publish response: %v body=%s", err, w2.Body.String())
	}

	if resp1.JobID == "" {
		t.Fatalf("expected jobId from first publish")
	}
	if resp2.JobID != resp1.JobID {
		t.Fatalf("expected duplicate publish to return same job id, first=%s second=%s", resp1.JobID, resp2.JobID)
	}
	if resp2.AlreadyEnqueued != true {
		t.Fatalf("expected second publish alreadyEnqueued=true, got=%v", resp2.AlreadyEnqueued)
	}

	var jobsCount int
	err = pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM jobs
		WHERE idempotency_key = $1
	`, "publish:event:"+eventID).Scan(&jobsCount)
	if err != nil {
		t.Fatalf("count publish jobs: %v", err)
	}
	if jobsCount != 1 {
		t.Fatalf("expected exactly one publish job row, got %d", jobsCount)
	}
}
