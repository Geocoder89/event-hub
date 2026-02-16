package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/geocoder89/eventhub/internal/domain/event"
	"github.com/geocoder89/eventhub/internal/http/handlers"
	"github.com/gin-gonic/gin"
)

type bindErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details struct {
			JSON   string                `json:"json"`
			Field  string                `json:"field"`
			Fields []handlers.FieldError `json:"fields"`
		} `json:"details"`
	} `json:"error"`
}

func TestBindJSON_ValidationErrorsUseJSONFieldNames(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/events", func(ctx *gin.Context) {
		var req event.CreateEventRequest
		if !handlers.BindJSON(ctx, &req) {
			return
		}
		ctx.Status(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(`{"title":"go"}`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}

	var resp bindErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v body=%s", err, w.Body.String())
	}

	if resp.Error.Code != "invalid_request" {
		t.Fatalf("unexpected code: %s", resp.Error.Code)
	}

	wantRules := map[string]string{
		"title":    "min",
		"startAt":  "required",
		"capacity": "required",
	}

	found := map[string]handlers.FieldError{}
	for _, fieldErr := range resp.Error.Details.Fields {
		found[fieldErr.Field] = fieldErr
	}

	for field, rule := range wantRules {
		fieldErr, ok := found[field]
		if !ok {
			t.Fatalf("missing field error for %q: %+v", field, resp.Error.Details.Fields)
		}
		if fieldErr.Rule != rule {
			t.Fatalf("field %q rule mismatch: got %q want %q", field, fieldErr.Rule, rule)
		}
		if fieldErr.Message == "" {
			t.Fatalf("field %q should include a non-empty message", field)
		}
	}
}

func TestBindJSON_TypeMismatchUsesJSONFieldNames(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.POST("/events", func(ctx *gin.Context) {
		var req event.CreateEventRequest
		if !handlers.BindJSON(ctx, &req) {
			return
		}
		ctx.Status(http.StatusCreated)
	})

	body := `{"title":"Go Meetup","startAt":"2026-03-01T09:00:00Z","capacity":"ten"}`
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}

	var resp bindErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v body=%s", err, w.Body.String())
	}

	if resp.Error.Details.JSON != "invalid_json_type" {
		t.Fatalf("expected invalid_json_type, got %q", resp.Error.Details.JSON)
	}
	if resp.Error.Details.Field != "capacity" {
		t.Fatalf("expected detail field to be capacity, got %q", resp.Error.Details.Field)
	}
	if len(resp.Error.Details.Fields) == 0 {
		t.Fatalf("expected at least one field error in details.fields")
	}

	fieldErr := resp.Error.Details.Fields[0]
	if fieldErr.Field != "capacity" {
		t.Fatalf("expected fields[0].field=capacity, got %q", fieldErr.Field)
	}
	if fieldErr.Rule != "type" {
		t.Fatalf("expected fields[0].rule=type, got %q", fieldErr.Rule)
	}
	if fieldErr.Message == "" {
		t.Fatalf("expected non-empty fields[0].message")
	}
}
