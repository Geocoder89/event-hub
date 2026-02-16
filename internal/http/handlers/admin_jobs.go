package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/domain/job"
	"github.com/geocoder89/eventhub/internal/http/middlewares"
	"github.com/geocoder89/eventhub/internal/repo/postgres"
	"github.com/geocoder89/eventhub/internal/utils"
	"github.com/gin-gonic/gin"
)

type AdminJobsRepo interface {
	ListCursor(
		ctx context.Context,
		status *string,
		limit int,
		afterUpdatedAt time.Time,
		afterID string,
	) (items []job.Job, nextCursor *string, hasMore bool, err error)
	GetByID(ctx context.Context, id string) (job.Job, error)
	Retry(ctx context.Context, id string) error
	RetryManyFailed(ctx context.Context, limit int) (int64, error)
}

type AdminJobsHandler struct {
	repo AdminJobsRepo
}

func NewAdminJobsHandler(repo AdminJobsRepo) *AdminJobsHandler {
	return &AdminJobsHandler{
		repo: repo,
	}
}

// func parseInt(s string, fallback int) int {
// 	if s == "" {
// 		return fallback
// 	}

// 	n, err := strconv.Atoi(s)

// 	if err != nil {
// 		return fallback
// 	}

// 	return n
// }

// Get /admin/jobs?status=failed&limit=50&offset=0

func (h *AdminJobsHandler) List(ctx *gin.Context) {
	limit := parseIntDefault(ctx.Query("limit"), 20)
	if limit < 1 || limit > 100 {
		RespondBadRequest(ctx, "invalid_query", "limit must be between 1 and 100")
		return
	}

	var statusPtr *string
	if s := ctx.Query("status"); s != "" {
		statusPtr = &s
	}

	cursor := ctx.Query("cursor")

	// DESC first-page sentinel: "far future" + max UUID
	afterUpdatedAt := time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
	afterID := "ffffffff-ffff-ffff-ffff-ffffffffffff"

	if cursor != "" {
		cur, err := utils.DecodeJobCursor(cursor)
		if err != nil {
			RespondBadRequest(ctx, "invalid_query", "cursor is invalid")
			return
		}
		afterUpdatedAt = cur.UpdatedAt
		afterID = cur.ID
	}

	cctx, cancel := config.WithTimeout(2 * time.Second)
	defer cancel()

	items, next, hasMore, err := h.repo.ListCursor(cctx, statusPtr, limit, afterUpdatedAt, afterID)
	if err != nil {
		RespondInternal(ctx, "Could not list jobs")
		return
	}

	resp := gin.H{
		"limit":      limit,
		"count":      len(items),
		"items":      items,
		"hasMore":    hasMore,
		"nextCursor": next,
	}

	RespondJSONWithETag(ctx, http.StatusOK, resp)
}

// Get /admin/jobs/:id

func (h *AdminJobsHandler) GetByID(ctx *gin.Context) {
	id := ctx.Param("id")
	ctx.Set(middlewares.CtxJobID, id)

	if !utils.IsUUID(id) {
		RespondBadRequest(ctx, "invalid_request", "invalid_id")
		return
	}

	cctx, cancel := config.WithTimeout(2 * time.Second)
	defer cancel()

	j, err := h.repo.GetByID(cctx, id)

	if err != nil {
		if errors.Is(err, job.ErrJobNotFound) {
			RespondNotFound(ctx, "Job not found")
			return
		}

		RespondInternal(ctx, "Could not fetch job")
		return
	}

	RespondJSONWithETag(ctx, http.StatusOK, j)
}

// POST /admin/jobs/:id/retry
func (h *AdminJobsHandler) Retry(ctx *gin.Context) {
	id := ctx.Param("id")
	ctx.Set(middlewares.CtxJobID, id)
	if !utils.IsUUID(id) {
		RespondBadRequest(ctx, "invalid_request", "invalid_id")
		return
	}

	cctx, cancel := config.WithTimeout(2 * time.Second)
	defer cancel()

	err := h.repo.Retry(cctx, id)
	if err != nil {
		if errors.Is(err, job.ErrJobNotFound) {
			RespondNotFound(ctx, "Job not found")
			return
		}
		if errors.Is(err, postgres.ErrJobNotFailed) {
			RespondConflict(ctx, "job_not_failed", "Only failed jobs can be retried")
		}
		RespondInternal(ctx, "Could not retry job")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"jobId":  id,
		"status": "pending",
	})
}

// POST /admin/jobs/reprocess-dead?limit=50

func (h *AdminJobsHandler) ReprocessDead(ctx *gin.Context) {
	limitStr := ctx.Query("limit")

	limit := 50

	if limitStr != "" {
		n, err := strconv.Atoi(limitStr)

		if err == nil {
			limit = n
		} else {
			RespondBadRequest(ctx, "invalid_request", "limit must be a number")
			return
		}
	}

	cctx, cancel := config.WithTimeout(3 * time.Second)

	defer cancel()

	n, err := h.repo.RetryManyFailed(cctx, limit)

	if err != nil {
		RespondInternal(ctx, "Could not reprocess dead jobs")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"requeued": n,
	})
}
