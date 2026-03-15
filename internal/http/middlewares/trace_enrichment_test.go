package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTraceEnrichment_AddsSpanAttributes(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	tr := tp.Tracer("test-trace-enrichment")

	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx, span := tr.Start(c.Request.Context(), "http.request")
		defer span.End()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(TraceEnrichment())

	r.GET("/events/:id/register", func(c *gin.Context) {
		c.Set(CtxRequestID, "req-123")
		c.Set(CtxUserID, "user-456")
		c.Set(CtxJobID, "job-789")
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/events/evt-42/register", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	got := attrsToMap(spans[0].Attributes())
	assertAttrValue(t, got, "request.id", "req-123")
	assertAttrValue(t, got, "user.id", "user-456")
	assertAttrValue(t, got, "job.id", "job-789")
	assertAttrValue(t, got, "event.id", "evt-42")
}

func TestTraceEnrichment_NoSpan_NoPanic(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(TraceEnrichment())
	r.GET("/events/:id", func(c *gin.Context) {
		c.Set(CtxRequestID, "req-1")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/events/evt-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func attrsToMap(attrs []attribute.KeyValue) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, kv := range attrs {
		out[string(kv.Key)] = kv.Value.AsString()
	}
	return out
}

func assertAttrValue(t *testing.T, got map[string]string, key, want string) {
	t.Helper()
	v, ok := got[key]
	if !ok {
		t.Fatalf("expected attribute %s to be present", key)
	}
	if v != want {
		t.Fatalf("expected %s=%q, got %q", key, want, v)
	}
}
