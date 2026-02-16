package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/geocoder89/eventhub/internal/cache"
	"github.com/geocoder89/eventhub/internal/domain/event"
	"github.com/geocoder89/eventhub/internal/http/handlers"
	"github.com/geocoder89/eventhub/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Make sure Gin does not spam the console during the test

func init() {
	gin.SetMode(gin.TestMode)
}

func newUUID() string {
	return uuid.NewString()
}

// Fake repository implementations of the handlers.EventCreator interface

type fakeEventsRepo struct {
	createFn     func(ctx context.Context, req event.CreateEventRequest) (event.Event, error)
	getFn        func(ctx context.Context, id string) (event.Event, error)
	listFn       func(ctx context.Context, filters event.ListEventsFilter) ([]event.Event, int, error)
	listCursorFn func(ctx context.Context, filters event.ListEventsFilter, afterStartAt time.Time, afterID string) ([]event.Event, *string, bool, error)
	countFn      func(ctx context.Context, filters event.ListEventsFilter) (int, error)
	updateFn     func(ctx context.Context, id string, req event.UpdateEventRequest) (event.Event, error)
	deleteFn     func(ctx context.Context, id string) error
}

func (f *fakeEventsRepo) Create(ctx context.Context, req event.CreateEventRequest) (event.Event, error) {
	if f.createFn != nil {
		return f.createFn(ctx, req)
	}

	return event.Event{}, nil
}

func (f *fakeEventsRepo) Count(ctx context.Context, filters event.ListEventsFilter) (int, error) {
	if f.countFn != nil {
		return f.countFn(ctx, filters)
	}
	return 0, nil
}

func (f *fakeEventsRepo) ListCursor(
	ctx context.Context,
	filters event.ListEventsFilter,
	afterStartAt time.Time,
	afterID string,
) ([]event.Event, *string, bool, error) {
	if f.listCursorFn != nil {
		return f.listCursorFn(ctx, filters, afterStartAt, afterID)
	}
	return []event.Event{}, nil, false, nil
}

func (f *fakeEventsRepo) GetByID(ctx context.Context, id string) (event.Event, error) {
	if f.getFn != nil {
		return f.getFn(ctx, id)
	}

	return event.Event{}, nil
}

func (f *fakeEventsRepo) List(ctx context.Context, filters event.ListEventsFilter) ([]event.Event, int, error) {
	if f.listFn != nil {
		return f.listFn(ctx, filters)
	}

	return nil, 0, nil
}

func (f *fakeEventsRepo) Update(ctx context.Context, id string, req event.UpdateEventRequest) (event.Event, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, id, req)
	}

	return event.Event{}, nil
}

func (f *fakeEventsRepo) Delete(ctx context.Context, id string) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, id)
	}

	return nil
}

// small helper function which returns the gin engine to mount one handler per test

func setupRouter(method, path string, h gin.HandlerFunc) *gin.Engine {
	r := gin.New()

	r.Handle(method, path, h)

	return r
}

// Create Event tests

func TestCreateEventHandler(t *testing.T) {
	now := time.Now().UTC()

	// basic nomenclature/structure of a test
	tests := []struct {
		name           string
		body           string
		repoSetUp      func(*fakeEventsRepo) // function which takes a pointer to the fake events repo.
		wantStatusCode int
	}{
		{
			name: "success",
			body: `{
				"title": "Go Meetup",
				"description": "Day 10 test",
				"city": "Toronto",
				"startAt": "` + now.Format(time.RFC3339) + `",
				"capacity": 50
			}`,

			repoSetUp: func(f *fakeEventsRepo) {
				f.createFn = func(ctx context.Context, req event.CreateEventRequest) (event.Event, error) {
					return event.Event{
						ID:          newUUID(),
						Title:       req.Title,
						Description: req.Description,
						City:        req.City,
						StartAt:     req.StartAt,
						Capacity:    req.Capacity,
						CreatedAt:   now,
						UpdatedAt:   now,
					}, nil
				}
			},
			wantStatusCode: http.StatusCreated,
		},
		// in the event that it is a bad request.
		{
			name: "validation_error",
			body: `{"title": ""}`, // indicating of an invalid or incomplete request payload
			repoSetUp: func(f *fakeEventsRepo) {
				// since it is an invalid request the repo should not be called.
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "repo_error",
			body: `{
				"title": "Go Meetup",
				"description": "Day 10 test",
				"City": "Toronto",
				"startAt": "` + now.Format(time.RFC3339) + `",
				"capacity": 50
			}`,
			repoSetUp: func(f *fakeEventsRepo) {
				f.createFn = func(ctx context.Context, req event.CreateEventRequest) (event.Event, error) {
					return event.Event{}, errors.New("db error")
				}
			},
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			fakeEventsRepo := &fakeEventsRepo{}

			if tt.repoSetUp != nil {
				tt.repoSetUp(fakeEventsRepo)
			}

			h := handlers.NewEventsHandler(fakeEventsRepo)

			r := setupRouter(http.MethodPost, "/events", h.CreateEvent)

			req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			// returns a new response recorder
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Fatalf("got status %d, want %d,body=%s", w.Code, tt.wantStatusCode, w.Body.String())
			}
		})
	}
}

// ---List event tests

func TestListEventsHandler(t *testing.T) {
	now := time.Now().UTC()

	// Create a REAL cursor your handler can decode.
	validCursor, err := utils.EncodeEventCursor(
		now.Add(-time.Minute),
		"e42b6ed3-0af3-49f0-9dcd-37aa7ed8c980",
	)
	if err != nil {
		t.Fatalf("failed to build cursor: %v", err)
	}

	const zeroUUID = "00000000-0000-0000-0000-000000000000"

	tests := []struct {
		name           string
		url            string
		repoSetup      func(*fakeEventsRepo)
		wantStatusCode int
		wantCount      int
	}{
		{
			name: "success_first_page_no_cursor",
			url:  "/events?limit=20",
			repoSetup: func(f *fakeEventsRepo) {
				f.listCursorFn = func(ctx context.Context, filters event.ListEventsFilter, afterStartAt time.Time, afterID string) ([]event.Event, *string, bool, error) {
					// Option A: first page uses epoch + zero UUID (per your handler).
					if !afterStartAt.Equal(time.Unix(0, 0).UTC()) {
						return nil, nil, false, errors.New("afterStartAt not epoch for first page")
					}
					if afterID != zeroUUID {
						return nil, nil, false, errors.New("afterID not zero UUID for first page")
					}

					next := "next-cursor"
					return []event.Event{
						{
							ID:          "id-1",
							Title:       "Event 1",
							Description: "Desc",
							City:        "Toronto",
							StartAt:     now,
							Capacity:    10,
							CreatedAt:   now,
							UpdatedAt:   now,
						},
					}, &next, true, nil
				}
			},
			wantStatusCode: http.StatusOK,
			wantCount:      1,
		},
		{
			name: "success_with_search_query",
			url:  "/events?limit=20&q=backend+go",
			repoSetup: func(f *fakeEventsRepo) {
				f.listCursorFn = func(ctx context.Context, filters event.ListEventsFilter, afterStartAt time.Time, afterID string) ([]event.Event, *string, bool, error) {
					if filters.Query == nil || *filters.Query != "backend go" {
						return nil, nil, false, errors.New("query filter not passed")
					}

					return []event.Event{
						{
							ID:          "id-search-1",
							Title:       "Go Backend Deep Dive",
							Description: "Search test",
							City:        "Lagos",
							StartAt:     now,
							Capacity:    80,
							CreatedAt:   now,
							UpdatedAt:   now,
						},
					}, nil, false, nil
				}
			},
			wantStatusCode: http.StatusOK,
			wantCount:      1,
		},

		{
			name: "success_with_valid_cursor",
			url:  "/events?limit=20&cursor=" + validCursor,
			repoSetup: func(f *fakeEventsRepo) {
				f.listCursorFn = func(ctx context.Context, filters event.ListEventsFilter, afterStartAt time.Time, afterID string) ([]event.Event, *string, bool, error) {
					// Cursor path: should be NOT epoch and NOT zero UUID.
					if afterStartAt.Equal(time.Unix(0, 0).UTC()) {
						return nil, nil, false, errors.New("afterStartAt should not be epoch when cursor provided")
					}
					if afterID == "" || afterID == zeroUUID {
						return nil, nil, false, errors.New("afterID should not be empty/zero UUID when cursor provided")
					}

					next := "next-cursor-2"
					return []event.Event{
						{
							ID:          "id-2",
							Title:       "Event 2",
							Description: "Desc",
							City:        "Toronto",
							StartAt:     now.Add(time.Hour),
							Capacity:    10,
							CreatedAt:   now,
							UpdatedAt:   now,
						},
					}, &next, true, nil
				}
			},
			wantStatusCode: http.StatusOK,
			wantCount:      1,
		},

		{
			name:           "invalid_cursor",
			url:            "/events?cursor=!!!", // valid URL, invalid base64url => should 400
			repoSetup:      nil,
			wantStatusCode: http.StatusBadRequest,
			wantCount:      0,
		},

		{
			name: "repo_error",
			url:  "/events?limit=20",
			repoSetup: func(f *fakeEventsRepo) {
				f.listCursorFn = func(ctx context.Context, filters event.ListEventsFilter, afterStartAt time.Time, afterID string) ([]event.Event, *string, bool, error) {
					return nil, nil, false, errors.New("db error")
				}
			},
			wantStatusCode: http.StatusInternalServerError,
			wantCount:      0,
		},

		// page param is ignored in cursor-only mode
		{
			name: "page_param_is_ignored",
			url:  "/events?page=0",
			repoSetup: func(f *fakeEventsRepo) {
				f.listCursorFn = func(ctx context.Context, filters event.ListEventsFilter, afterStartAt time.Time, afterID string) ([]event.Event, *string, bool, error) {
					return []event.Event{}, nil, false, nil
				}
			},
			wantStatusCode: http.StatusOK,
			wantCount:      0,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			fakeRepo := &fakeEventsRepo{}
			if tt.repoSetup != nil {
				tt.repoSetup(fakeRepo)
			}

			h := handlers.NewEventsHandler(fakeRepo)
			r := setupRouter(http.MethodGet, "/events", h.ListEvents)

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Fatalf("got status %d, want %d, body=%s", w.Code, tt.wantStatusCode, w.Body.String())
			}

			if tt.wantStatusCode == http.StatusOK {
				var resp struct {
					Count int `json:"count"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}
				if resp.Count != tt.wantCount {
					t.Fatalf("got count %d, want %d", resp.Count, tt.wantCount)
				}
			}
		})
	}
}

func TestUpdateEventHandler(t *testing.T) {
	now := time.Now().UTC()
	validID := newUUID()
	missingID := newUUID()

	tests := []struct {
		name           string
		body           string
		url            string
		repoSetup      func(f *fakeEventsRepo)
		wantStatusCode int
	}{
		{
			name: "success",
			url:  "/events/" + validID,
			body: `{
			"title": "Updated Title",
			"description": "Updated description",
			"city": "Toronto",
			"startAt": "` + now.Format(time.RFC3339) + `",
			"capacity": 100
			}`,
			repoSetup: func(f *fakeEventsRepo) {
				f.updateFn = func(ctx context.Context, id string, req event.UpdateEventRequest) (event.Event, error) {
					return event.Event{
						ID:          id,
						Title:       req.Title,
						Description: req.Description,
						City:        req.City,
						StartAt:     req.StartAt,
						Capacity:    req.Capacity,
						CreatedAt:   now.Add(-time.Hour),
						UpdatedAt:   now,
					}, nil
				}
			},
			wantStatusCode: http.StatusOK,
		},

		// in the event there was no event with the ID found

		{
			name: "not_found",
			url:  "/events/" + missingID,
			body: `{
				"title": "Updated Title",
				"description": "Updated Desc",
				"city": "Toronto",
				"startAt": "` + now.Format(time.RFC3339) + `",
				"capacity": 100
			}`,
			repoSetup: func(f *fakeEventsRepo) {
				f.updateFn = func(ctx context.Context, id string, req event.UpdateEventRequest) (event.Event, error) {
					return event.Event{}, event.ErrNotFound
				}
			},
			wantStatusCode: http.StatusNotFound,
		},

		// invalid payload.
		{
			name: "validation_error",
			url:  "/events/" + validID,
			body: `{"title": ""}`, // invalid payload
			repoSetup: func(f *fakeEventsRepo) {
				// repo should not be called at all in this case.
			},
			wantStatusCode: http.StatusBadRequest,
		},

		// db error

		{
			name: "repo_error",
			url:  "/events/" + validID,
			body: `{
				"title": "Updated Title",
				"description": "Updated Desc",
				"city": "Toronto",
				"startAt": "` + now.Format(time.RFC3339) + `",
				"capacity": 100
			}`,
			repoSetup: func(f *fakeEventsRepo) {
				f.updateFn = func(ctx context.Context, id string, req event.UpdateEventRequest) (event.Event, error) {
					return event.Event{}, errors.New("db error")
				}
			},
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			fakeEventsRepo := &fakeEventsRepo{}

			if tt.repoSetup != nil {
				tt.repoSetup(fakeEventsRepo)
			}

			h := handlers.NewEventsHandler(fakeEventsRepo)

			r := setupRouter(http.MethodPut, "/events/:id", h.UpdateEvent)
			req := httptest.NewRequest(http.MethodPut, tt.url, bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Fatalf("got status %d, want %d, body=%s", w.Code, tt.wantStatusCode, w.Body.String())
			}
		})
	}
}

func TestGetEventByIdHandler(t *testing.T) {
	now := time.Now().UTC()
	validID := newUUID()
	missingID := newUUID()

	tests := []struct {
		name           string
		url            string
		repoSetup      func(f *fakeEventsRepo)
		wantStatusCode int
	}{
		{
			name: "success",
			url:  "/events/" + validID,
			repoSetup: func(f *fakeEventsRepo) {
				f.getFn = func(ctx context.Context, id string) (event.Event, error) {
					return event.Event{
						ID:          id,
						Title:       "Event-1",
						Description: "Desc",
						City:        "Toronto",
						StartAt:     now,
						Capacity:    10,
						CreatedAt:   now.Add(-time.Hour),
						UpdatedAt:   now,
					}, nil
				}
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "not_found",
			url:  "/events/" + missingID,
			repoSetup: func(f *fakeEventsRepo) {
				f.getFn = func(ctx context.Context, id string) (event.Event, error) {
					return event.Event{}, event.ErrNotFound
				}
			},
			wantStatusCode: http.StatusNotFound,
		},
		{
			name: "repo_error",
			url:  "/events/" + validID,
			repoSetup: func(f *fakeEventsRepo) {
				f.getFn = func(ctx context.Context, id string) (event.Event, error) {
					return event.Event{}, errors.New("db error")
				}
			},
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			fakeEventsRepo := &fakeEventsRepo{}

			if tt.repoSetup != nil {
				tt.repoSetup(fakeEventsRepo)
			}

			h := handlers.NewEventsHandler(fakeEventsRepo)
			r := setupRouter(http.MethodGet, "/events/:id", h.GetEventById)

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Fatalf("got status %d, want %d, body=%s", w.Code, tt.wantStatusCode, w.Body.String())
			}
		})
	}
}

func TestDeleteEventHandler(t *testing.T) {

	validID := newUUID()
	missingID := newUUID()
	tests := []struct {
		name           string
		url            string
		repoSetup      func(*fakeEventsRepo)
		wantStatusCode int
	}{
		{
			name: "success",
			url:  "/events/" + validID,
			repoSetup: func(f *fakeEventsRepo) {
				f.deleteFn = func(ctx context.Context, id string) error {
					return nil
				}
			},

			wantStatusCode: http.StatusNoContent,
		},

		{
			name: "not_found",
			url:  "/events/" + missingID,
			repoSetup: func(f *fakeEventsRepo) {
				f.deleteFn = func(ctx context.Context, id string) error {
					return event.ErrNotFound
				}
			},
			wantStatusCode: http.StatusNotFound,
		},
		{
			name: "repo_error",
			url:  "/events/" + validID,
			repoSetup: func(f *fakeEventsRepo) {
				f.deleteFn = func(ctx context.Context, id string) error {
					return errors.New("db error")
				}
			},
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			fakeEventsRepo := &fakeEventsRepo{}

			if tt.repoSetup != nil {
				tt.repoSetup(fakeEventsRepo)
			}

			h := handlers.NewEventsHandler(fakeEventsRepo)

			r := setupRouter(http.MethodDelete, "/events/:id", h.DeleteEvent)

			req := httptest.NewRequest(http.MethodDelete, tt.url, nil)

			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Fatalf("got status %d, want %d, body=%s", w.Code, tt.wantStatusCode, w.Body.String())
			}
		})
	}
}

func TestListEventsHandler_CacheHit(t *testing.T) {
	now := time.Now().UTC()
	const zeroUUID = "00000000-0000-0000-0000-000000000000"

	fakeRepo := &fakeEventsRepo{}
	c := cache.New(30 * time.Second)

	calls := 0
	fakeRepo.listCursorFn = func(ctx context.Context, filters event.ListEventsFilter, afterStartAt time.Time, afterID string) ([]event.Event, *string, bool, error) {
		calls++
		if !afterStartAt.Equal(time.Unix(0, 0).UTC()) {
			return nil, nil, false, errors.New("afterStartAt not epoch")
		}
		if afterID != zeroUUID {
			return nil, nil, false, errors.New("afterID not zero uuid")
		}
		return []event.Event{
			{ID: "id-1", Title: "Event 1", City: "Toronto", StartAt: now, CreatedAt: now, UpdatedAt: now},
		}, nil, false, nil
	}

	h := handlers.NewEventsHandlerWithCache(fakeRepo, c)
	r := setupRouter(http.MethodGet, "/events", h.ListEvents)

	// First request: cache miss -> repo called
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/events?limit=20", nil)
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("first call got %d body=%s", w1.Code, w1.Body.String())
	}

	// Second request: cache hit -> repo should NOT be called again
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/events?limit=20", nil)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("second call got %d body=%s", w2.Code, w2.Body.String())
	}

	if calls != 1 {
		t.Fatalf("expected repo calls=1, got %d", calls)
	}
}

func TestListEventsHandler_ETagNotModified(t *testing.T) {
	now := time.Now().UTC()
	const zeroUUID = "00000000-0000-0000-0000-000000000000"

	fakeRepo := &fakeEventsRepo{}
	c := cache.New(30 * time.Second)
	calls := 0

	fakeRepo.listCursorFn = func(ctx context.Context, filters event.ListEventsFilter, afterStartAt time.Time, afterID string) ([]event.Event, *string, bool, error) {
		calls++
		if !afterStartAt.Equal(time.Unix(0, 0).UTC()) {
			return nil, nil, false, errors.New("afterStartAt not epoch")
		}
		if afterID != zeroUUID {
			return nil, nil, false, errors.New("afterID not zero uuid")
		}

		return []event.Event{
			{ID: "id-1", Title: "Event 1", City: "Toronto", StartAt: now, CreatedAt: now, UpdatedAt: now},
		}, nil, false, nil
	}

	h := handlers.NewEventsHandlerWithCache(fakeRepo, c)
	r := setupRouter(http.MethodGet, "/events", h.ListEvents)

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/events?limit=20", nil)
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("first call got %d body=%s", w1.Code, w1.Body.String())
	}

	etag := w1.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("expected ETag header in first response")
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/events?limit=20", nil)
	req2.Header.Set("If-None-Match", etag)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotModified {
		t.Fatalf("second call got %d, want %d, body=%s", w2.Code, http.StatusNotModified, w2.Body.String())
	}

	if w2.Body.Len() != 0 {
		t.Fatalf("expected empty body for 304, got %q", w2.Body.String())
	}

	if got := w2.Header().Get("ETag"); got == "" {
		t.Fatalf("expected ETag header in 304 response")
	}

	if calls != 1 {
		t.Fatalf("expected repo calls=1 due cache hit, got %d", calls)
	}
}

func TestGetEventByIDHandler_ETagNotModified(t *testing.T) {
	now := time.Now().UTC()
	validID := newUUID()

	fakeRepo := &fakeEventsRepo{}
	calls := 0

	fakeRepo.getFn = func(ctx context.Context, id string) (event.Event, error) {
		calls++
		return event.Event{
			ID:          id,
			Title:       "Event-1",
			Description: "Desc",
			City:        "Toronto",
			StartAt:     now,
			Capacity:    10,
			CreatedAt:   now.Add(-time.Hour),
			UpdatedAt:   now,
		}, nil
	}

	h := handlers.NewEventsHandler(fakeRepo)
	r := setupRouter(http.MethodGet, "/events/:id", h.GetEventById)

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/events/"+validID, nil)
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("first call got %d body=%s", w1.Code, w1.Body.String())
	}

	etag := w1.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("expected ETag header in first response")
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/events/"+validID, nil)
	req2.Header.Set("If-None-Match", etag)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotModified {
		t.Fatalf("second call got %d, want %d, body=%s", w2.Code, http.StatusNotModified, w2.Body.String())
	}

	if w2.Body.Len() != 0 {
		t.Fatalf("expected empty body for 304, got %q", w2.Body.String())
	}

	if calls != 2 {
		t.Fatalf("expected repo to be called on each lookup, got %d calls", calls)
	}
}
