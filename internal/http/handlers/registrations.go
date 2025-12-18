package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/domain/registration"
	"github.com/gin-gonic/gin"
)
type RegistrationCreator interface {
	Create(ctx context.Context, req registration.CreateRegistrationRequest)(registration.Registration, error)
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

	if !BindJSON(ctx,&req) {
		return
	}

	// force URL param as the source of truth

	req.EventID = eventID


	cctx, cancel := config.WithTimeout(2 * time.Second)

	defer cancel()


	reg, err := h.repo.Create(cctx,req)

	if err != nil {
		if errors.Is(err,registration.ErrAlreadyRegistered){
			RespondConflict(ctx,"already_registered","this email is already registered for this event.")
			return
		}

		// if the event is full spring up an error from  the db
		if errors.Is(err, registration.ErrEventFull){
			RespondConflict(ctx,"event_full","this event is already at full capacity.")
			return
		}

		fmt.Println(err)
		// otherwise return 500

		RespondInternal(ctx,"Could not register for event")
		return
	}

	ctx.JSON(http.StatusCreated,reg)
}