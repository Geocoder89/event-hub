package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/domain/job"
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

type JobsHandler struct {
	jobs JobsCreator
}

func NewJobsHandler(jobsRepo JobsCreator) *JobsHandler {
	return &JobsHandler{jobs: jobsRepo}
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
