package middlewares

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		method      string
		body        string
		contentType string
		wantStatus  int
	}{
		{
			name:       "post_without_body_allows_missing_content_type",
			method:     http.MethodPost,
			body:       "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "post_with_body_requires_json_content_type",
			method:     http.MethodPost,
			body:       `{"ok":true}`,
			wantStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:        "post_with_body_and_json_content_type_passes",
			method:      http.MethodPost,
			body:        `{"ok":true}`,
			contentType: "application/json",
			wantStatus:  http.StatusOK,
		},
		{
			name:       "put_without_body_allows_missing_content_type",
			method:     http.MethodPut,
			body:       "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "get_is_not_checked",
			method:     http.MethodGet,
			body:       "",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(RequireJSON())
			r.Any("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

			req := httptest.NewRequest(tt.method, "/x", strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("got %d want %d body=%s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
