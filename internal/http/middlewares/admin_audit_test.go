package middlewares

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type capturedAuditEntry struct {
	actorUserID  string
	actorEmail   string
	actorRole    string
	action       string
	resourceType string
	resourceID   string
	requestID    string
	statusCode   int
	details      map[string]any
}

type fakeAdminAuditWriter struct {
	entries []capturedAuditEntry
	err     error
}

func (f *fakeAdminAuditWriter) Write(
	ctx context.Context,
	actorUserID, actorEmail, actorRole, action, resourceType, resourceID, requestID string,
	statusCode int,
	details map[string]any,
) error {
	f.entries = append(f.entries, capturedAuditEntry{
		actorUserID:  actorUserID,
		actorEmail:   actorEmail,
		actorRole:    actorRole,
		action:       action,
		resourceType: resourceType,
		resourceID:   resourceID,
		requestID:    requestID,
		statusCode:   statusCode,
		details:      details,
	})

	return f.err
}

func TestAdminAudit_WritesForMutatingAdminRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	writer := &fakeAdminAuditWriter{}
	r := gin.New()

	r.Use(func(c *gin.Context) {
		c.Set(CtxUserID, "admin-user-1")
		c.Set(CtxEmail, "admin@example.com")
		c.Set(CtxRole, "admin")
		c.Set(CtxRequestID, "req-123")
		c.Next()
	})
	r.Use(AdminAudit(writer))

	r.POST("/admin/events/:id/publish", func(c *gin.Context) {
		c.Set(CtxJobID, "job-123")
		c.JSON(http.StatusAccepted, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/events/evt-1/publish?runAt=2026-02-15T12:00:00Z", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusAccepted)
	}

	if len(writer.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(writer.entries))
	}

	got := writer.entries[0]
	if got.actorUserID != "admin-user-1" {
		t.Fatalf("actor user id mismatch: got %q", got.actorUserID)
	}
	if got.actorEmail != "admin@example.com" {
		t.Fatalf("actor email mismatch: got %q", got.actorEmail)
	}
	if got.actorRole != "admin" {
		t.Fatalf("actor role mismatch: got %q", got.actorRole)
	}
	if got.requestID != "req-123" {
		t.Fatalf("request id mismatch: got %q", got.requestID)
	}
	if got.action != "POST /admin/events/:id/publish" {
		t.Fatalf("action mismatch: got %q", got.action)
	}
	if got.resourceType != "events" {
		t.Fatalf("resource type mismatch: got %q", got.resourceType)
	}
	if got.resourceID != "evt-1" {
		t.Fatalf("resource id mismatch: got %q", got.resourceID)
	}
	if got.statusCode != http.StatusAccepted {
		t.Fatalf("status code mismatch: got %d", got.statusCode)
	}
	if got.details["jobId"] != "job-123" {
		t.Fatalf("expected details.jobId=job-123, got %+v", got.details["jobId"])
	}
}

func TestAdminAudit_SkipsGetRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	writer := &fakeAdminAuditWriter{}
	r := gin.New()

	r.Use(func(c *gin.Context) {
		c.Set(CtxUserID, "admin-user-1")
		c.Set(CtxEmail, "admin@example.com")
		c.Set(CtxRole, "admin")
		c.Set(CtxRequestID, "req-123")
		c.Next()
	})
	r.Use(AdminAudit(writer))

	r.GET("/admin/jobs", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/jobs", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}

	if len(writer.entries) != 0 {
		t.Fatalf("expected 0 audit entries for GET, got %d", len(writer.entries))
	}
}

func TestAdminAudit_WriteErrorDoesNotBreakResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	writer := &fakeAdminAuditWriter{err: errors.New("db down")}
	r := gin.New()

	r.Use(func(c *gin.Context) {
		c.Set(CtxUserID, "admin-user-1")
		c.Set(CtxEmail, "admin@example.com")
		c.Set(CtxRole, "admin")
		c.Set(CtxRequestID, "req-123")
		c.Next()
	})
	r.Use(AdminAudit(writer))

	r.DELETE("/admin/events/:id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodDelete, "/admin/events/evt-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusNoContent)
	}

	if len(writer.entries) != 1 {
		t.Fatalf("expected attempted audit write, got %d entries", len(writer.entries))
	}
}
