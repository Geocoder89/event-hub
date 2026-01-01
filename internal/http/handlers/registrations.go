package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/domain/event"
	"github.com/geocoder89/eventhub/internal/domain/registration"
	"github.com/geocoder89/eventhub/internal/http/middlewares"
	"github.com/gin-gonic/gin"
)

type RegistrationCreator interface {
	Create(ctx context.Context, req registration.CreateRegistrationRequest) (registration.Registration, error)
	ListByEvent(ctx context.Context, eventID string) ([]registration.Registration, error)
	GetByID(ctx context.Context, eventID, registrationID string) (registration.Registration, error)
	Delete(ctx context.Context, eventID, registrationID string) error
}

type RegistrationHandler struct {
	repo RegistrationCreator
}

func NewRegistrationHandler(repo RegistrationCreator) *RegistrationHandler {
	return &RegistrationHandler{repo: repo}
}

func (h *RegistrationHandler) Register(ctx *gin.Context) {
	eventID := ctx.Param("id")

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

	reg, err := h.repo.Create(cctx, req)

	if err != nil {
		if errors.Is(err, registration.ErrAlreadyRegistered) {
			RespondConflict(ctx, "already_registered", "this email is already registered for this event.")
			return
		}

		// if the event is full spring up an error from  the db
		if errors.Is(err, registration.ErrEventFull) {
			RespondConflict(ctx, "event_full", "this event is already at full capacity.")
			return
		}
		// otherwise return 500

		RespondInternal(ctx, "Could not register for event")
		fmt.Println(err)
		return
	}

	ctx.JSON(http.StatusCreated, reg)
}

func (h *RegistrationHandler) ListForEvent(ctx *gin.Context) {
	eventID := ctx.Param("id")

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
