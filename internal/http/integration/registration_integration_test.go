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
	"testing"
	"time"

	"github.com/geocoder89/eventhub/internal/config"
	apphttp "github.com/geocoder89/eventhub/internal/http"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testConfig() config.Config {
	return config.Config{
		Env:                 "test",
		Port:                0,                   // not used in tests
		DBURL:               "",                  // pool created manually in tests
		AdminEmail:          "admin@example.com", // not used here
		AdminPassword:       "ignored-in-tests",
		AdminName:           "Test Admin",
		AdminRole:           "admin",
		JWTSecret:           "test-secret-key", // deterministic test secret
		JWTAccessTTLMinutes: 60,
	}
}

type apiErrorResponse struct {
	Error struct {
		Code      string          `json:"code"`
		Message   string          `json:"message"`
		RequestID string          `json:"requestId"`
		Details   json.RawMessage `json:"details"`
	} `json:"error"`
}

func setupTestRouter(t *testing.T) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		// default for local dev (your docker-compose)
		dsn = "postgres://eventhub:eventhub@127.0.0.1:5433/eventhub?sslmode=disable"
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)

	if err != nil {
		t.Fatalf("Failed to create pgx pool: %v", err)
	}
	// Basic logger that discards outputs during tests

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	cfg := testConfig()

	router := apphttp.NewRouter(logger, pool, cfg)

	return router, pool
}

// reset db function after every test

func resetDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	// Truncate in dependency order noting that registrations depend on events

	_, err := pool.Exec(context.Background(), `TRUNCATE events RESTART IDENTITY CASCADE`)

	if err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}
}

// Create a seeded event for our integration tests

func seedEvent(t *testing.T, pool *pgxpool.Pool, capacity int) string {
	t.Helper()
	id := uuid.NewString()
	now := time.Now().UTC()
	startAt := now.Add(24 * time.Hour) // start at is 24 hours from our current time.

	_, err := pool.Exec(
		context.Background(),
		`INSERT INTO events (id, title, description, city, start_at, capacity, created_at, updated_at)
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		id,
		"Test Event",
		"Integration test event",
		"Toronto",
		startAt,
		capacity,
		now,
		now,
	)

	if err != nil {
		t.Fatalf("failed to insert seed event: %v", err)
	}

	return id
}

func TestRegisterIntegration_HappyPath(t *testing.T) {
	// instantiate the test router
	router, pool := setupTestRouter(t)

	//  for each run make sure, there are no data in the db.
	resetDB(t, pool)
	defer resetDB(t, pool)
	eventID := seedEvent(t, pool, 2)

	body := `{
			"name": "Sam Doe",
			"email": "sam@example.com"
	 }`

	req := httptest.NewRequest(http.MethodPost, "/events/"+eventID+"/register", bytes.NewBufferString(body))

	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusCreated, w.Body.String())
	}

	//  we can also verify if the row exists

	var count int
	err := pool.QueryRow(
		context.Background(),
		`SELECT COUNT(*) FROM registrations WHERE event_id = $1 AND email = $2`,
		eventID,
		"sam@example.com",
	).Scan(&count)

	if err != nil {
		t.Fatalf("failed to query registrations: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 registration, got %d", count)
	}
}

// integration test to check if there are duplicate emails

func TestRegisterIntegration_DuplicateEmail(t *testing.T) {
	router, pool := setupTestRouter(t)
	resetDB(t, pool)
	defer resetDB(t, pool)

	eventID := seedEvent(t, pool, 2)

	body := `{
			"name": "Sam Doe",
			"email": "sam@example.com"
	 }`

	//  first registration should succeed
	req1 := httptest.NewRequest(http.MethodPost, "/events/"+eventID+"/register", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("[first call] got status %d, want %d, body=%s", w1.Code, http.StatusCreated, w1.Body.String())
	}

	// second registration with the same email should record already registered with email

	req2 := httptest.NewRequest(http.MethodPost, "/events/"+eventID+"/register", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("[second call] got status %d, want %d, body=%s", w2.Code, http.StatusConflict, w2.Body.String())
	}

	var response apiErrorResponse
	err := json.Unmarshal(w2.Body.Bytes(), &response)

	if err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)

	}

	if response.Error.Code != "already_registered" {
		t.Fatalf("expected error code 'already registered' got '%s'", response.Error.Code)
	}
}

func TestRegisterIntegration_EventFull(t *testing.T) {
	router, pool := setupTestRouter(t)

	resetDB(t, pool)
	defer resetDB(t, pool)
	// capacity = 1
	eventID := seedEvent(t, pool, 1)

	body1 := `{"name":"User One","email":"user1@example.com"}`
	body2 := `{"name":"User Two","email":"user2@example.com"}`

	// First registration (fills capacity)
	req1 := httptest.NewRequest(http.MethodPost, "/events/"+eventID+"/register", bytes.NewBufferString(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("[first call] got status %d, want %d, body=%s", w1.Code, http.StatusCreated, w1.Body.String())
	}

	// Second registration (different email) -> should get event_full
	req2 := httptest.NewRequest(http.MethodPost, "/events/"+eventID+"/register", bytes.NewBufferString(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("[second call] got status %d, want %d, body=%s", w2.Code, http.StatusConflict, w2.Body.String())
	}

	var resp apiErrorResponse
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if resp.Error.Code != "event_full" {
		t.Fatalf("expected error code 'event_full', got '%s'", resp.Error.Code)
	}
}

func TestRegisterIntegration_EventNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)

	body := `{"name":"Sam Example","email":"sam@example.com"}`

	nonExistentID := uuid.NewString()

	req := httptest.NewRequest(http.MethodPost, "/events/"+nonExistentID+"/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound && w.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, expected 404 or 500 depending on mapping, body=%s", w.Code, w.Body.String())
	}

}
