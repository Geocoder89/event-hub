package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/domain/job"
	"github.com/geocoder89/eventhub/internal/repo/postgres"
	"github.com/geocoder89/eventhub/internal/utils"
	"github.com/gin-gonic/gin"
)

type AdminJobsRepo interface {
	List(ctx context.Context, status *string, limit, offset int) ([]job.Job, error)
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

func parseInt(s string, fallback int) int {
	if s == "" {
		return fallback
	}

	n, err := strconv.Atoi(s)

	if err != nil {
		return fallback
	}

	return n
}

// Get /admin/jobs?status=failed&limit=50&offset=0

func (h *AdminJobsHandler) List(ctx *gin.Context) {
	limit := parseInt(ctx.Query("limit"), 50)
	offset := parseInt(ctx.Query("offset"), 0)

	if limit < 1 || limit > 200 {
		RespondBadRequest(ctx, "invalid_query", "limit must be between 1 and 200")
		return
	}

	if offset < 0 {
		RespondBadRequest(ctx, "invalid_query", "offset must be >= 0")
		return
	}

	var statusPointer *string
	s := ctx.Query("status")

	if s != "" {
		statusPointer = &s
	}

	cctx, cancel := config.WithTimeout(2 * time.Second)

	defer cancel()

	items, err := h.repo.List(cctx, statusPointer, limit, offset)

	if err != nil {
		fmt.Println(err)
		RespondInternal(ctx, "Could not list jobs")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"limit":  limit,
		"offset": offset,
		"count":  len(items),
		"items":  items,
	})
}

// Get /admin/jobs/:id

func (h *AdminJobsHandler) GetByID(ctx *gin.Context) {
	id := ctx.Param("id")

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

	ctx.JSON(http.StatusOK, j)
}

// POST /admin/jobs/:id/retry
func (h *AdminJobsHandler) Retry(ctx *gin.Context) {
	id := ctx.Param("id")
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
