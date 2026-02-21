package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/geocoder89/eventhub/internal/domain/job"
	"github.com/geocoder89/eventhub/internal/http/handlers"
	"github.com/gin-gonic/gin"
)

type fakeAdminJobsRepo struct {
	listCursorFn      func(ctx context.Context, status *string, limit int, afterUpdatedAt time.Time, afterID string) ([]job.Job, *string, bool, error)
	countFn           func(ctx context.Context, status *string) (int, error)
	getByIDFn         func(ctx context.Context, id string) (job.Job, error)
	retryFn           func(ctx context.Context, id string) error
	retryManyFailedFn func(ctx context.Context, limit int) (int64, error)
}

func (f *fakeAdminJobsRepo) ListCursor(ctx context.Context, status *string, limit int, afterUpdatedAt time.Time, afterID string) ([]job.Job, *string, bool, error) {
	if f.listCursorFn != nil {
		return f.listCursorFn(ctx, status, limit, afterUpdatedAt, afterID)
	}
	return nil, nil, false, nil
}

func (f *fakeAdminJobsRepo) Count(ctx context.Context, status *string) (int, error) {
	if f.countFn != nil {
		return f.countFn(ctx, status)
	}
	return 0, nil
}

func (f *fakeAdminJobsRepo) GetByID(ctx context.Context, id string) (job.Job, error) {
	if f.getByIDFn != nil {
		return f.getByIDFn(ctx, id)
	}
	return job.Job{}, nil
}

func (f *fakeAdminJobsRepo) Retry(ctx context.Context, id string) error {
	if f.retryFn != nil {
		return f.retryFn(ctx, id)
	}
	return nil
}

func (f *fakeAdminJobsRepo) RetryManyFailed(ctx context.Context, limit int) (int64, error) {
	if f.retryManyFailedFn != nil {
		return f.retryManyFailedFn(ctx, limit)
	}
	return 0, nil
}

func TestAdminJobsList_IncludeTotal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &fakeAdminJobsRepo{}
	countCalls := 0

	repo.listCursorFn = func(ctx context.Context, status *string, limit int, afterUpdatedAt time.Time, afterID string) ([]job.Job, *string, bool, error) {
		return []job.Job{}, nil, false, nil
	}
	repo.countFn = func(ctx context.Context, status *string) (int, error) {
		countCalls++
		return 7, nil
	}

	h := handlers.NewAdminJobsHandler(repo)
	r := gin.New()
	r.GET("/admin/jobs", h.List)

	req := httptest.NewRequest(http.MethodGet, "/admin/jobs?limit=20&includeTotal=true", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp struct {
		Total *int `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Total == nil || *resp.Total != 7 {
		t.Fatalf("expected total=7, got %+v", resp.Total)
	}
	if countCalls != 1 {
		t.Fatalf("expected count calls=1 got=%d", countCalls)
	}
}

func TestAdminJobsList_WithoutIncludeTotal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &fakeAdminJobsRepo{}
	countCalls := 0

	repo.listCursorFn = func(ctx context.Context, status *string, limit int, afterUpdatedAt time.Time, afterID string) ([]job.Job, *string, bool, error) {
		return []job.Job{}, nil, false, nil
	}
	repo.countFn = func(ctx context.Context, status *string) (int, error) {
		countCalls++
		return 7, nil
	}

	h := handlers.NewAdminJobsHandler(repo)
	r := gin.New()
	r.GET("/admin/jobs", h.List)

	req := httptest.NewRequest(http.MethodGet, "/admin/jobs?limit=20", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp struct {
		Total *int `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Total != nil {
		t.Fatalf("expected total=nil got=%v", *resp.Total)
	}
	if countCalls != 0 {
		t.Fatalf("expected count calls=0 got=%d", countCalls)
	}
}

func TestAdminJobsList_CountError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &fakeAdminJobsRepo{}
	repo.listCursorFn = func(ctx context.Context, status *string, limit int, afterUpdatedAt time.Time, afterID string) ([]job.Job, *string, bool, error) {
		return []job.Job{}, nil, false, nil
	}
	repo.countFn = func(ctx context.Context, status *string) (int, error) {
		return 0, errors.New("db")
	}

	h := handlers.NewAdminJobsHandler(repo)
	r := gin.New()
	r.GET("/admin/jobs", h.List)

	req := httptest.NewRequest(http.MethodGet, "/admin/jobs?limit=20&includeTotal=true", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}
