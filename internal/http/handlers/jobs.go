package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/domain/job"
	"github.com/geocoder89/eventhub/internal/domain/registrationexport"
	"github.com/geocoder89/eventhub/internal/http/middlewares"
	"github.com/geocoder89/eventhub/internal/jobs"
	"github.com/geocoder89/eventhub/internal/repo/postgres"
	"github.com/jackc/pgx/v5"

	"github.com/geocoder89/eventhub/internal/utils"
	"github.com/gin-gonic/gin"
)

type JobsCreator interface {
	Create(ctx context.Context, req job.CreateRequest) (job.Job, error)
	CreateTx(ctx context.Context, tx pgx.Tx, req job.CreateRequest) (job.Job, error)
	GetByIdempotencyKey(ctx context.Context, key string) (job.Job, error)
}

type RegistrationCSVExportsReader interface {
	GetByJobID(ctx context.Context, jobID string) (registrationexport.CSVExport, error)
}

type JobsHandler struct {
	jobs    JobsCreator
	exports RegistrationCSVExportsReader
}

func NewJobsHandler(jobsRepo JobsCreator, exportsRepo RegistrationCSVExportsReader) *JobsHandler {
	return &JobsHandler{jobs: jobsRepo, exports: exportsRepo}
}

// POST /events/:id/publish

func (h *JobsHandler) PublishEvent(ctx *gin.Context) {
	eventID := ctx.Param("id")

	runAt := time.Now().UTC()

	if !utils.IsUUID(eventID) {
		RespondBadRequest(ctx, "invalid_request", "invalid_id")
		return
	}

	userID, ok := middlewares.UserIDFromContext(ctx)

	runAtStr := ctx.Query("runAt")

	if runAtStr != "" {
		t, err := time.Parse(time.RFC3339, runAtStr)

		if err != nil {
			RespondBadRequest(ctx, "invalid_query", "runAt must be RFC 3339 Datetime")
			return
		}

		// small guard: allow slight clock drift but reject clearly-in-the-past schedules
		if t.Before(time.Now().UTC().Add(-30 * time.Second)) {
			RespondBadRequest(ctx, "invalid_query", "runAt must be now or in the future")
			return
		}

		runAt = t.UTC()
	}

	if !ok || userID == "" {
		RespondUnAuthorized(ctx, "unauthorized", "Missing identity")
		return
	}

	payload := jobs.EventPublishPayload{
		EventID:     eventID,
		RequestedBy: userID,
		RequestedAt: time.Now().UTC(),
		RequestID:   requestIDFrom(ctx),
	}

	raw, err := payload.ToJSONRaw()

	if err != nil {
		RespondInternal(ctx, "Could not enqueue job")
		return
	}

	cctx, cancel := config.WithTimeout(2 * time.Second)

	defer cancel()
	key := "publish:event:" + eventID

	j, err := h.jobs.Create(cctx, job.CreateRequest{
		Type:           jobs.TypeEventPublish,
		Payload:        json.RawMessage(raw),
		RunAt:          runAt,
		MaxAttempts:    25,
		IdempotencyKey: &key,
		UserID:         &userID,
	})

	if err != nil {
		if postgres.IsUniqueViolation(err) {
			existing, gerr := h.jobs.GetByIdempotencyKey(cctx, key)

			if gerr != nil {
				RespondInternal(ctx, "Could not enqueue job")
			}

			ctx.JSON(http.StatusAccepted, gin.H{
				"jobId":           existing.ID,
				"status":          existing.Status,
				"type":            existing.Type,
				"alreadyEnqueued": true,
			})
			ctx.Set(middlewares.CtxJobID, existing.ID)
			slog.Default().InfoContext(cctx, "job.enqueue",
				"request_id", requestIDFrom(ctx),
				"job_id", existing.ID,
				"job_type", existing.Type,
				"already_enqueued", true,
			)

			return

		}

		RespondInternal(ctx, "Could not enqueue job")
		return
	}

	ctx.JSON(http.StatusAccepted, gin.H{
		"jobId":  j.ID,
		"status": j.Status,
		"type":   j.Type,
	})
	ctx.Set(middlewares.CtxJobID, j.ID)
	slog.Default().InfoContext(cctx, "job.enqueue",
		"request_id", requestIDFrom(ctx),
		"job_id", j.ID,
		"job_type", j.Type,
		"already_enqueued", false,
	)

}

// POST /events/:id/registrations/export
func (h *JobsHandler) ExportRegistrationsCSV(ctx *gin.Context) {
	eventID := ctx.Param("id")
	if !utils.IsUUID(eventID) {
		RespondBadRequest(ctx, "invalid_request", "invalid_id")
		return
	}

	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok || userID == "" {
		RespondUnAuthorized(ctx, "unauthorized", "Missing identity")
		return
	}

	payload := jobs.RegistrationsExportCSVPayload{
		EventID:     eventID,
		RequestedBy: userID,
		RequestedAt: time.Now().UTC(),
		RequestID:   requestIDFrom(ctx),
	}

	raw, err := payload.JSON()
	if err != nil {
		RespondInternal(ctx, "Could not enqueue job")
		return
	}

	cctx, cancel := config.WithTimeout(2 * time.Second)
	defer cancel()

	j, err := h.jobs.Create(cctx, job.CreateRequest{
		Type:        jobs.TypeRegistrationsExportCSV,
		Payload:     json.RawMessage(raw),
		RunAt:       time.Now().UTC(),
		MaxAttempts: 10,
		UserID:      &userID,
		Priority:    1,
	})
	if err != nil {
		RespondInternal(ctx, "Could not enqueue job")
		return
	}

	ctx.Set(middlewares.CtxJobID, j.ID)
	slog.Default().InfoContext(cctx, "job.enqueue",
		"request_id", requestIDFrom(ctx),
		"job_id", j.ID,
		"job_type", j.Type,
		"already_enqueued", false,
	)

	ctx.JSON(http.StatusAccepted, gin.H{
		"jobId":           j.ID,
		"status":          j.Status,
		"type":            j.Type,
		"downloadPath":    "/admin/jobs/" + j.ID + "/registrations-export.csv",
		"alreadyEnqueued": false,
	})
}

// GET /admin/jobs/:id/registrations-export.csv
func (h *JobsHandler) DownloadRegistrationsCSV(ctx *gin.Context) {
	jobID := ctx.Param("id")
	if !utils.IsUUID(jobID) {
		RespondBadRequest(ctx, "invalid_request", "invalid_id")
		return
	}

	if h.exports == nil {
		RespondInternal(ctx, "Registrations export store not configured")
		return
	}

	cctx, cancel := config.WithTimeout(2 * time.Second)
	defer cancel()

	exported, err := h.exports.GetByJobID(cctx, jobID)
	if err != nil {
		if errors.Is(err, registrationexport.ErrNotFound) {
			RespondNotFound(ctx, "Export not found")
			return
		}
		RespondInternal(ctx, "Could not fetch export")
		return
	}

	contentType := exported.ContentType
	if contentType == "" {
		contentType = "text/csv"
	}

	fileName := exported.FileName
	if fileName == "" {
		fileName = "registrations.csv"
	}

	ctx.Header("Content-Type", contentType)
	ctx.Header("Content-Disposition", `attachment; filename="`+fileName+`"`)
	ctx.Header("X-Export-Row-Count", strconv.Itoa(exported.RowCount))
	ctx.Data(http.StatusOK, contentType, exported.Data)
}
