package integration__test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/geocoder89/eventhub/internal/config"
	apphttp "github.com/geocoder89/eventhub/internal/http"
	"github.com/geocoder89/eventhub/internal/notifications"
	"github.com/geocoder89/eventhub/internal/queue/worker"
	"github.com/geocoder89/eventhub/internal/repo/postgres"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type recordingNotifier struct {
	mu    sync.Mutex
	calls []notifications.SendRegistrationConfirmationInput
}

func (n *recordingNotifier) SendRegistrationConfirmation(ctx context.Context, input notifications.SendRegistrationConfirmationInput) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, input)
	return nil
}

func (n *recordingNotifier) Count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.calls)
}

func (n *recordingNotifier) Last() (notifications.SendRegistrationConfirmationInput, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.calls) == 0 {
		return notifications.SendRegistrationConfirmationInput{}, false
	}
	return n.calls[len(n.calls)-1], true
}

func setupPipelineRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool, config.Config) {
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
		JWTSecret:           "test-secret-key",
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
		TRUNCATE
			notification_deliveries,
			refresh_tokens,
			registrations,
			jobs,
			events,
			users
		RESTART IDENTITY CASCADE
	`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func TestPipeline_Register_EnqueuesJob_Worker_SendsOnce(t *testing.T) {
	router, pool, _ := setupPipelineRouter(t)
	resetPipelineDB(t, pool)
	defer resetPipelineDB(t, pool)

	// 1) Seed event (reusing my seedEvent helper; ensuring it inserts a valid UUID)
	eventID := seedEvent(t, pool, 10)

	// 2) Signup user and call /events/:id/register (API step)
	userEmail := "pipeline-user@example.com"
	token := signupAndGetToken(t, router, userEmail) // using my signup and get token function from prior integration tests.

	registerBody := `{"name":"Pipeline User","email":"` + userEmail + `"}`

	req := httptest.NewRequest(http.MethodPost, "/events/"+eventID+"/register", bytes.NewBufferString(registerBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("register got %d body=%s", w.Code, w.Body.String())
	}

	// Parse registration id out of response ( handler returns reg)
	var reg struct {
		ID      string `json:"id"`
		EventID string `json:"eventId"`
		Email   string `json:"email"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &reg); err != nil {
		t.Fatalf("parse register resp: %v body=%s", err, w.Body.String())
	}
	if reg.ID == "" {
		t.Fatalf("expected registration id in response")
	}

	// 3) Assert job exists in DB (Job step)
	var jobID string
	var jobType string
	var status string
	var jobUserID *string

	err := pool.QueryRow(context.Background(), `
		SELECT id, type, status, user_id
		FROM jobs
		WHERE type = 'registration.confirmation'
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(&jobID, &jobType, &status, &jobUserID)

	if err != nil {
		t.Fatalf("select job: %v", err)
	}
	if jobID == "" || jobType != "registration.confirmation" {
		t.Fatalf("expected confirmation job, got id=%s type=%s", jobID, jobType)
	}
	if status != "pending" && status != "processing" {
		t.Fatalf("expected job pending/processing, got %s", status)
	}
	if jobUserID == nil || *jobUserID == "" {
		t.Fatalf("expected job.user_id to be set (day 51 propagation)")
	}

	// 4) Run worker once (Worker step)
	jobsRepo := postgres.NewJobsRepo(pool)
	eventsRepo := postgres.NewEventsRepo(pool)
	deliveriesRepo := postgres.NewNotificationsDeliveriesRepo(pool)

	rec := &recordingNotifier{}

	wk := worker.New(worker.Config{
		PollInterval:  10 * time.Millisecond,
		WorkerID:      "test-worker",
		Concurrency:   1,
		ShutdownGrace: 1 * time.Second,
	}, jobsRepo, eventsRepo, rec, deliveriesRepo)

	processed, err := wk.ProcessOne(context.Background())
	if err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}
	if !processed {
		t.Fatalf("expected a job to be processed")
	}

	// 5) Assert side-effect (notification_deliveries sent + notifier called exactly once)
	var deliveryStatus string
	var sentAt *time.Time
	err = pool.QueryRow(context.Background(), `
		SELECT status, sent_at
		FROM notification_deliveries
		WHERE kind = 'registration.confirmation' AND registration_id = $1
	`, reg.ID).Scan(&deliveryStatus, &sentAt)
	if err != nil {
		t.Fatalf("select notification delivery: %v", err)
	}
	if deliveryStatus != "sent" || sentAt == nil {
		t.Fatalf("expected delivery sent, got status=%s sentAt=%v", deliveryStatus, sentAt)
	}

	if rec.Count() != 1 {
		t.Fatalf("expected notifier to be called once, got %d", rec.Count())
	}
	last, ok := rec.Last()
	if !ok {
		t.Fatalf("expected last notifier call")
	}
	if last.Email != userEmail || last.RegistrationID != reg.ID || last.EventID != eventID {
		t.Fatalf("unexpected notifier payload: %+v", last)
	}

	// 6) (Optionally) Run worker again; verify no second send (idempotency)
	processed2, err := wk.ProcessOne(context.Background())
	if err != nil {
		t.Fatalf("ProcessOne(2): %v", err)
	}
	_ = processed2 // might be false; depends on whether more jobs exist
	if rec.Count() != 1 {
		t.Fatalf("expected still 1 notifier call after re-run, got %d", rec.Count())
	}
}
