package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/domain/job"
	"github.com/geocoder89/eventhub/internal/http/middlewares"
	"github.com/geocoder89/eventhub/internal/jobs"
	"github.com/geocoder89/eventhub/internal/utils"
	"github.com/gin-gonic/gin"
)

type JobsCreator interface {
	Create(ctx context.Context, req job.CreateRequest) (job.Job, error)
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

	if !utils.IsUUID(eventID) {
		RespondBadRequest(ctx, "invalid_request", "invalid_id")
		return
	}

	userID, ok := middlewares.UserIDFromContext(ctx)

	if !ok || userID == "" {
		RespondUnAuthorized(ctx, "unauthorized", "Missing identity")
	}

	payload := jobs.EventPublishPayload{
		EventID:     eventID,
		RequestedBy: userID,
		RequestedAt: time.Now().UTC(),
	}

	raw, err := payload.ToJSONRaw()

	if err != nil {
		RespondInternal(ctx, "Could not enqueue job")
		return
	}

	cctx, cancel := config.WithTimeout(2 * time.Second)

	defer cancel()

	j, err := h.jobs.Create(cctx, job.CreateRequest{
		Type:        jobs.TypeEventPublish,
		Payload:     json.RawMessage(raw),
		RunAt:       time.Now().UTC(),
		MaxAttempts: 25,
	})

	if err != nil {
		RespondInternal(ctx, "Could not enqueue job")
		return
	}

	ctx.JSON(http.StatusAccepted, gin.H{
		"jobId":  j.ID,
		"status": j.Status,
		"type":   j.Type,
	})
}
