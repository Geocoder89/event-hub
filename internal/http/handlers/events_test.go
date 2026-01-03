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

	"github.com/geocoder89/eventhub/internal/domain/event"
	"github.com/geocoder89/eventhub/internal/http/handlers"
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
	createFn func(ctx context.Context, req event.CreateEventRequest) (event.Event, error)
	getFn    func(ctx context.Context, id string) (event.Event, error)
	listFn   func(ctx context.Context, filters event.ListEventsFilter) ([]event.Event, int, error)
	updateFn func(ctx context.Context, id string, req event.UpdateEventRequest) (event.Event, error)
	deleteFn func(ctx context.Context, id string) error
}

func (f *fakeEventsRepo) Create(ctx context.Context, req event.CreateEventRequest) (event.Event, error) {
	if f.createFn != nil {
		return f.createFn(ctx, req)
	}

	return event.Event{}, nil
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
				"City": "Toronto",
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
	// validId := newUUID()

	tests := []struct {
		name           string
		url            string
		repoSetup      func(*fakeEventsRepo)
		wantStatusCode int
		wantCount      int
	}{
		{
			name: "success_basic",
			url:  "/events",
			repoSetup: func(f *fakeEventsRepo) {
				f.listFn = func(ctx context.Context, filters event.ListEventsFilter) ([]event.Event, int, error) {
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
					}, 1, nil
				}

			},
			wantStatusCode: http.StatusOK,
			wantCount:      1,
		},

		{
			name:           "invalid_page",
			url:            "/events?page=0",
			repoSetup:      nil,
			wantStatusCode: http.StatusBadRequest,
			wantCount:      0,
		},
		{
			name: "repo_error",
			url:  "/events",
			repoSetup: func(f *fakeEventsRepo) {
				f.listFn = func(ctx context.Context, filters event.ListEventsFilter) ([]event.Event, int, error) {
					return []event.Event{}, 0, errors.New("db error")
				}
			},
			wantStatusCode: http.StatusInternalServerError,
			wantCount:      0,
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

				err := json.Unmarshal(w.Body.Bytes(), &resp)

				if err != nil {
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
