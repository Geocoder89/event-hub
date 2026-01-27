package handlers

import (
	"context"
	"errors"
	"fmt"
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

	_, err = h.jobsRepo.CreateTx(cctx, tx, job.CreateRequest{
		Type:           jobs.TypeRegistrationConfirmation,
		Payload:        raw,
		RunAt:          time.Now().UTC(),
		MaxAttempts:    10,
		IdempotencyKey: &key,
		UserID: &uid,
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
	ctx.JSON(http.StatusCreated, reg)
}

func (h *RegistrationHandler) ListForEvent(ctx *gin.Context) {
	eventID := ctx.Param("id")

	if !utils.IsUUID(eventID) {
		RespondBadRequest(ctx, "invalid_id", "event id must be a valid UUID")
		return
	}

	cctx, cancel := config.WithTimeout(2 * time.Second)
	defer cancel()

	regs, err := h.repo.ListByEvent(cctx, eventID)
	if err != nil {
		if errors.Is(err, event.ErrNotFound) {
			RespondNotFound(ctx, "Event not found")
			return
		}

		RespondInternal(ctx, "Could not list registrations")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"eventId":       eventID,
		"count":         len(regs),
		"registrations": regs,
	})
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
