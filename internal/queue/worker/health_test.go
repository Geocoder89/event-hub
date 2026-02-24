package worker

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthHandlerReadyz(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		ready      bool
		check      func() error
		wantStatus int
	}{
		{
			name:       "not ready flag returns 503",
			ready:      false,
			check:      nil,
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "ready with no checker returns 200",
			ready:      true,
			check:      nil,
			wantStatus: http.StatusOK,
		},
		{
			name:  "ready with failing checker returns 503",
			ready: true,
			check: func() error {
				return errors.New("db down")
			},
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:  "ready with successful checker returns 200",
			ready: true,
			check: func() error {
				return nil
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := &Worker{ready: tc.ready}
			if tc.check != nil {
				w.WithReadinessCheck(func(context.Context) error {
					return tc.check()
				})
			}

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			w.HealthHandler(nil).ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", rr.Code, tc.wantStatus, rr.Body.String())
			}
		})
	}
}
