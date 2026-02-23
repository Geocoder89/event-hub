package integration__test

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/geocoder89/eventhub/internal/notifications"
	"github.com/geocoder89/eventhub/internal/queue/worker"
	"github.com/geocoder89/eventhub/internal/repo/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func seedUserForExport(t *testing.T, pool *pgxpool.Pool, id, email, name string) {
	t.Helper()

	now := time.Now().UTC()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO users (id, email, password_hash, name, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, id, email, "test-hash", name, "user", now, now)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func seedRegistrationForExport(
	t *testing.T,
	pool *pgxpool.Pool,
	id string,
	eventID string,
	userID string,
	name string,
	email string,
	token string,
	createdAt time.Time,
) {
	t.Helper()

	_, err := pool.Exec(context.Background(), `
		INSERT INTO registrations (
			id, event_id, user_id, name, email, check_in_token, checked_in_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, NULL, $7, $7
		)
	`, id, eventID, userID, name, email, token, createdAt.UTC())
	if err != nil {
		t.Fatalf("seed registration: %v", err)
	}
}

func TestPipeline_RegistrationsCSVExport_EnqueueProcessDownload(t *testing.T) {
	router, pool, _ := setupPipelineRouter(t)
	resetPipelineDB(t, pool)
	defer resetPipelineDB(t, pool)

	eventID := seedEvent(t, pool, 50)
	adminToken := createAdminAuthToken(t, router, pool, "admin-export@example.com")

	user1ID := uuid.NewString()
	user2ID := uuid.NewString()
	seedUserForExport(t, pool, user1ID, "csv-user1@example.com", "CSV User One")
	seedUserForExport(t, pool, user2ID, "csv-user2@example.com", "CSV User Two")

	now := time.Now().UTC().Truncate(time.Second)
	seedRegistrationForExport(
		t, pool,
		uuid.NewString(), eventID, user1ID,
		"CSV User One", "csv-user1@example.com", "chk_token_one",
		now,
	)
	seedRegistrationForExport(
		t, pool,
		uuid.NewString(), eventID, user2ID,
		"CSV User Two", "csv-user2@example.com", "chk_token_two",
		now.Add(1*time.Second),
	)

	enqueueW := doAuthedJSONRequest(
		router,
		http.MethodPost,
		"/admin/events/"+eventID+"/registrations/export",
		`{}`,
		adminToken,
	)
	if enqueueW.Code != http.StatusAccepted {
		t.Fatalf("enqueue got status=%d body=%s", enqueueW.Code, enqueueW.Body.String())
	}

	var enqueueResp struct {
		JobID           string `json:"jobId"`
		Status          string `json:"status"`
		Type            string `json:"type"`
		DownloadPath    string `json:"downloadPath"`
		AlreadyEnqueued bool   `json:"alreadyEnqueued"`
	}
	if err := json.Unmarshal(enqueueW.Body.Bytes(), &enqueueResp); err != nil {
		t.Fatalf("decode enqueue response: %v body=%s", err, enqueueW.Body.String())
	}
	if enqueueResp.JobID == "" {
		t.Fatalf("expected jobId in enqueue response")
	}
	if enqueueResp.Type != "registrations.export_csv" {
		t.Fatalf("expected type registrations.export_csv got %q", enqueueResp.Type)
	}
	if !strings.Contains(enqueueResp.DownloadPath, enqueueResp.JobID) {
		t.Fatalf("downloadPath does not include job id: %q", enqueueResp.DownloadPath)
	}
	if enqueueResp.AlreadyEnqueued {
		t.Fatalf("expected alreadyEnqueued=false for fresh export job")
	}

	jobsRepo := postgres.NewJobsRepo(pool, nil)
	eventsRepo := postgres.NewEventsRepo(pool, nil)
	regsRepo := postgres.NewRegistrationsRepo(pool, nil)
	exportsRepo := postgres.NewRegistrationCSVExportsRepo(pool)
	deliveriesRepo := postgres.NewNotificationsDeliveriesRepo(pool)

	wk := worker.New(worker.Config{
		PollInterval:  10 * time.Millisecond,
		WorkerID:      "test-worker-export-csv",
		Concurrency:   1,
		ShutdownGrace: 1 * time.Second,
	}, jobsRepo, eventsRepo, notifications.NewLogNotifier(), deliveriesRepo).
		WithRegistrationCSVExporter(regsRepo, exportsRepo)

	processed, err := wk.ProcessOne(context.Background())
	if err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}
	if !processed {
		t.Fatalf("expected one job to be processed")
	}

	var dbCount int
	var dbContentType string
	var dbFileName string
	var dbCSV []byte
	err = pool.QueryRow(context.Background(), `
		SELECT row_count, content_type, file_name, csv_data
		FROM registration_csv_exports
		WHERE job_id = $1
	`, enqueueResp.JobID).Scan(&dbCount, &dbContentType, &dbFileName, &dbCSV)
	if err != nil {
		t.Fatalf("select export row: %v", err)
	}
	if dbCount != 2 {
		t.Fatalf("expected row_count=2 got %d", dbCount)
	}
	if dbContentType != "text/csv" {
		t.Fatalf("expected content_type text/csv got %q", dbContentType)
	}
	if dbFileName == "" {
		t.Fatalf("expected file_name to be set")
	}
	if len(dbCSV) == 0 {
		t.Fatalf("expected non-empty csv_data")
	}

	downloadW := doAuthedJSONRequest(
		router,
		http.MethodGet,
		"/admin/jobs/"+enqueueResp.JobID+"/registrations-export.csv",
		"",
		adminToken,
	)
	if downloadW.Code != http.StatusOK {
		t.Fatalf("download got status=%d body=%s", downloadW.Code, downloadW.Body.String())
	}

	if ct := downloadW.Header().Get("Content-Type"); ct != "text/csv" {
		t.Fatalf("expected content-type text/csv got %q", ct)
	}
	if got := downloadW.Header().Get("X-Export-Row-Count"); got != "2" {
		t.Fatalf("expected X-Export-Row-Count=2 got %q", got)
	}
	if cd := downloadW.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment;") || !strings.Contains(cd, ".csv") {
		t.Fatalf("unexpected Content-Disposition: %q", cd)
	}

	records, err := csv.NewReader(strings.NewReader(downloadW.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse csv response: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 csv rows (header + 2), got %d", len(records))
	}

	wantHeader := []string{
		"registration_id",
		"event_id",
		"user_id",
		"name",
		"email",
		"check_in_token",
		"checked_in_at",
		"created_at",
	}
	gotHeader := strings.Join(records[0], ",")
	if gotHeader != strings.Join(wantHeader, ",") {
		t.Fatalf("unexpected csv header: got=%q want=%q", gotHeader, strings.Join(wantHeader, ","))
	}

	emails := map[string]bool{}
	for _, row := range records[1:] {
		if len(row) != len(wantHeader) {
			t.Fatalf("unexpected csv column count=%d row=%v", len(row), row)
		}
		emails[row[4]] = true
	}
	if !emails["csv-user1@example.com"] || !emails["csv-user2@example.com"] {
		t.Fatalf("expected exported emails not found: %+v", emails)
	}
}
