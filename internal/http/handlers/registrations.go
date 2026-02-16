package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/domain/event"
	"github.com/geocoder89/eventhub/internal/domain/job"
	"github.com/geocoder89/eventhub/internal/domain/registration"
	"github.com/geocoder89/eventhub/internal/http/middlewares"
	"github.com/geocoder89/eventhub/internal/jobs"
	"github.com/geocoder89/eventhub/internal/repo/postgres"
	"github.com/geocoder89/eventhub/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type RegistrationCreator interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	CreateTx(ctx context.Context, tx pgx.Tx, req registration.CreateRegistrationRequest) (registration.Registration, error)
	Create(ctx context.Context, req registration.CreateRegistrationRequest) (registration.Registration, error)
	ListByEvent(ctx context.Context, eventID string) ([]registration.Registration, error)
	ListByEventCursor(
		ctx context.Context,
		eventID string,
		limit int,
		afterCreatedAt time.Time,
		afterID string,
	) (items []registration.Registration, nextCursor *string, hasMore bool, err error)

	CountForEvent(ctx context.Context, eventID string) (int, error)
	GetByID(ctx context.Context, eventID, registrationID string) (registration.Registration, error)
	Delete(ctx context.Context, eventID, registrationID string) error
}

type RegistrationHandler struct {
	repo     RegistrationCreator
	jobsRepo JobsCreator
}

func NewRegistrationHandler(repo RegistrationCreator, jobsRepo JobsCreator) *RegistrationHandler {
	return &RegistrationHandler{repo: repo, jobsRepo: jobsRepo}
}

func (h *RegistrationHandler) Register(ctx *gin.Context) {
	eventID := ctx.Param("id")

	if !utils.IsUUID(eventID) {
		RespondBadRequest(ctx, "invalid_id", "event id must be a valid UUID")
		return
	}

	var req registration.CreateRegistrationRequest

	if !BindJSON(ctx, &req) {
		return
	}

	// force URL param as the source of truth

	req.EventID = eventID

	// attach the userId to request.
	userID, ok := middlewares.UserIDFromContext(ctx)

	if !ok || userID == "" {
		RespondUnAuthorized(ctx, "unauthorized", "Missing identity")
		return
	}

	req.UserID = userID

	cctx, cancel := config.WithTimeout(2 * time.Second)

	defer cancel()

	tx, err := h.repo.BeginTx(cctx)

	if err != nil {
		RespondInternal(ctx, "Could not register for event")
		return
	}

	defer func() { _ = tx.Rollback(cctx) }()

	reg, err := h.repo.CreateTx(cctx, tx, req)

	if err != nil {
		switch {
		case errors.Is(err, registration.ErrAlreadyRegistered):
			RespondConflict(ctx, "already_registered", "this email is already registered for this event.")
		case errors.Is(err, registration.ErrEventFull):
			RespondConflict(ctx, "event_full", "this event is already at full capacity.")
		case errors.Is(err, event.ErrNotFound):
			RespondNotFound(ctx, "Event not found")
		default:
			RespondInternal(ctx, "Could not register for event")
			fmt.Println(err)
		}
		return
	}

	payload := jobs.RegistrationConfirmationPayload{
		RegistrationID: reg.ID,
		EventID:        reg.EventID,
		Email:          reg.Email,
		Name:           reg.Name,
		RequestedAt:    time.Now().UTC(),
		RequestID:      requestIDFrom(ctx),
	}

	raw, err := payload.JSON()

	if err != nil {
		RespondInternal(ctx, "Could not register for event")
		fmt.Println(err)
		return
	}

	// idempotency key
	key := "registration:confirm:" + reg.ID

	//capture userID as a variable so we can take its address
	uid := userID

	createdJob, err := h.jobsRepo.CreateTx(cctx, tx, job.CreateRequest{
		Type:           jobs.TypeRegistrationConfirmation,
		Payload:        raw,
		RunAt:          time.Now().UTC(),
		MaxAttempts:    10,
		IdempotencyKey: &key,
		UserID:         &uid,
	})
	if err != nil {
		// if duplicate idempotency key inside same tx, treat as OK (rare, but safe)
		if !postgres.IsUniqueViolation(err) {
			RespondInternal(ctx, "Could not register for event")
			fmt.Println(err)
			return
		}
	}

	// Commit once
	err = tx.Commit(cctx)
	if err != nil {
		RespondInternal(ctx, "Could not register for event")
		fmt.Println(err)
		return
	}
	if createdJob.ID != "" {
		ctx.Set(middlewares.CtxJobID, createdJob.ID)
		slog.Default().InfoContext(cctx, "job.enqueue",
			"request_id", requestIDFrom(ctx),
			"job_id", createdJob.ID,
			"job_type", createdJob.Type,
			"already_enqueued", false,
		)
	}
	ctx.JSON(http.StatusCreated, reg)
}

func (h *RegistrationHandler) ListForEvent(ctx *gin.Context) {
	eventID := ctx.Param("id")

	fmt.Println(utils.IsUUID(eventID), "it is")
	fmt.Println(eventID)

	if !utils.IsUUID(eventID) {
		RespondBadRequest(ctx, "invalid_id", "id must be a valid UUID")
		return
	}

	limit := parseIntDefault(ctx.Query("limit"), 20)
	if limit < 1 || limit > 100 {
		RespondBadRequest(ctx, "invalid_query", "limit must be between 1 and 100")
		return
	}

	includeTotal := ctx.Query("includeTotal") == "true"
	cursor := ctx.Query("cursor")

	afterCreatedAt := time.Unix(0, 0).UTC()
	afterID := "00000000-0000-0000-0000-000000000000"

	if cursor != "" {
		cur, err := utils.DecodeRegistrationCursor(cursor)
		if err != nil {
			RespondBadRequest(ctx, "invalid_query", "cursor is invalid")
			return
		}
		afterCreatedAt = cur.CreatedAt
		afterID = cur.ID
	}

	cctx, cancel := config.WithTimeout(2 * time.Second)
	defer cancel()

	items, next, hasMore, err := h.repo.ListByEventCursor(cctx, eventID, limit, afterCreatedAt, afterID)
	if err != nil {
		RespondInternal(ctx, "Could not list registrations")
		return
	}

	var total any = nil
	if includeTotal {
		t, err := h.repo.CountForEvent(cctx, eventID)
		if err != nil {
			RespondInternal(ctx, "Could not count registrations")
			return
		}
		total = t
	}

	resp := gin.H{
		"limit":      limit,
		"count":      len(items),
		"items":      items,
		"hasMore":    hasMore,
		"nextCursor": next,
		"total":      total,
	}

	RespondJSONWithETag(ctx, http.StatusOK, resp)
}

func (h *RegistrationHandler) Cancel(ctx *gin.Context) {
	eventID := ctx.Param("id")
	regID := ctx.Param("registrationId")

	if !utils.IsUUID(eventID) {
		RespondBadRequest(ctx, "invalid_id", "event id must be a valid UUID")
		return
	}

	if !utils.IsUUID(regID) {
		RespondBadRequest(ctx, "invalid_id", "registration id must be a valid UUID")
		return
	}

	// attach userID into request
	userID, ok := middlewares.UserIDFromContext(ctx)

	if !ok || userID == "" {
		RespondUnAuthorized(ctx, "unauthorized", "Missing identity")
		return
	}

	role, _ := middlewares.RoleFromContext(ctx)

	cctx, cancel := config.WithTimeout(2 * time.Second)
	defer cancel()

	// Load registration to check ownership

	reg, err := h.repo.GetByID(cctx, eventID, regID)

	if err != nil {
		if errors.Is(err, registration.ErrNotFound) {
			RespondNotFound(ctx, "Registration not found")

			return
		}

		RespondInternal(ctx, "Could not cancel registration")
		return
	}

	// Check ownership (admin override)

	if role != "admin" && reg.UserID != userID {
		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"code":    "forbidden",
				"message": "You can only cancel your registration",
			},
		})

		return
	}
	// Else delete
	err = h.repo.Delete(cctx, eventID, regID)
	if err != nil {
		if errors.Is(err, registration.ErrNotFound) {
			RespondNotFound(ctx, "Registration not found")
			return
		}

		RespondInternal(ctx, "Could not cancel registration")
		return
	}

	ctx.Status(http.StatusNoContent)
}
