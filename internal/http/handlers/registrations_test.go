package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/geocoder89/eventhub/internal/domain/job"
	"github.com/geocoder89/eventhub/internal/domain/registration"
	"github.com/geocoder89/eventhub/internal/http/handlers"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type fakeRegistrationsRepo struct {
	listByEventCursorFn func(ctx context.Context, eventID string, limit int, afterCreatedAt time.Time, afterID string) ([]registration.Registration, *string, bool, error)
	countForEventFn     func(ctx context.Context, eventID string) (int, error)
}

func (f *fakeRegistrationsRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return nil, nil
}

func (f *fakeRegistrationsRepo) CreateTx(ctx context.Context, tx pgx.Tx, req registration.CreateRegistrationRequest) (registration.Registration, error) {
	return registration.Registration{}, nil
}

func (f *fakeRegistrationsRepo) Create(ctx context.Context, req registration.CreateRegistrationRequest) (registration.Registration, error) {
	return registration.Registration{}, nil
}

func (f *fakeRegistrationsRepo) ListByEvent(ctx context.Context, eventID string) ([]registration.Registration, error) {
	return nil, nil
}

func (f *fakeRegistrationsRepo) ListByEventCursor(ctx context.Context, eventID string, limit int, afterCreatedAt time.Time, afterID string) ([]registration.Registration, *string, bool, error) {
	if f.listByEventCursorFn != nil {
		return f.listByEventCursorFn(ctx, eventID, limit, afterCreatedAt, afterID)
	}
	return nil, nil, false, nil
}

func (f *fakeRegistrationsRepo) CountForEvent(ctx context.Context, eventID string) (int, error) {
	if f.countForEventFn != nil {
		return f.countForEventFn(ctx, eventID)
	}
	return 0, nil
}

func (f *fakeRegistrationsRepo) GetByID(ctx context.Context, eventID, registrationID string) (registration.Registration, error) {
	return registration.Registration{}, nil
}

func (f *fakeRegistrationsRepo) Delete(ctx context.Context, eventID, registrationID string) error {
	return nil
}

func (f *fakeRegistrationsRepo) CheckInByToken(ctx context.Context, eventID, token string) (registration.Registration, error) {
	return registration.Registration{}, nil
}

type fakeJobsCreator struct{}

func (f *fakeJobsCreator) Create(ctx context.Context, req job.CreateRequest) (job.Job, error) {
	return job.Job{}, nil
}

func (f *fakeJobsCreator) CreateTx(ctx context.Context, tx pgx.Tx, req job.CreateRequest) (job.Job, error) {
	return job.Job{}, nil
}

func (f *fakeJobsCreator) GetByIdempotencyKey(ctx context.Context, key string) (job.Job, error) {
	return job.Job{}, nil
}

func TestRegistrationListForEvent_ETagNotModified(t *testing.T) {
	gin.SetMode(gin.TestMode)

	eventID := newUUID()
	now := time.Now().UTC()
	registrationID := newUUID()
	userID := newUUID()
	repoCalls := 0
	repo := &fakeRegistrationsRepo{}

	repo.listByEventCursorFn = func(ctx context.Context, gotEventID string, limit int, afterCreatedAt time.Time, afterID string) ([]registration.Registration, *string, bool, error) {
		repoCalls++
		if gotEventID != eventID {
			t.Fatalf("unexpected event id: %s", gotEventID)
		}
		if limit != 20 {
			t.Fatalf("expected limit=20, got %d", limit)
		}
		if !afterCreatedAt.Equal(time.Unix(0, 0).UTC()) {
			t.Fatalf("expected afterCreatedAt epoch, got %s", afterCreatedAt)
		}
		if afterID != "00000000-0000-0000-0000-000000000000" {
			t.Fatalf("unexpected afterID: %s", afterID)
		}

		return []registration.Registration{
			{
				ID:        registrationID,
				EventID:   eventID,
				UserID:    userID,
				Name:      "Test User",
				Email:     "test@example.com",
				CreatedAt: now,
				UpdatedAt: now,
			},
		}, nil, false, nil
	}

	h := handlers.NewRegistrationHandler(repo, &fakeJobsCreator{})
	r := gin.New()
	r.GET("/events/:id/registrations", h.ListForEvent)

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/events/"+eventID+"/registrations?limit=20", nil)
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("first call got %d, want %d, body=%s", w1.Code, http.StatusOK, w1.Body.String())
	}

	var responseBody map[string]any
	if err := json.Unmarshal(w1.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("decode first response: %v", err)
	}

	etag := w1.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("expected ETag header in first response")
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/events/"+eventID+"/registrations?limit=20", nil)
	req2.Header.Set("If-None-Match", etag)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotModified {
		t.Fatalf("second call got %d, want %d, body=%s", w2.Code, http.StatusNotModified, w2.Body.String())
	}

	if w2.Body.Len() != 0 {
		t.Fatalf("expected empty body for 304, got %q", w2.Body.String())
	}

	if repoCalls != 2 {
		t.Fatalf("expected repo calls=2, got %d", repoCalls)
	}
}
