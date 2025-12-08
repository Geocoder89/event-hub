package handlers

import (
	"net/http"

	"github.com/geocoder89/eventhub/internal/domain/event"
	"github.com/gin-gonic/gin"
)




type EventsCreator interface {
	Create(req event.CreateEventRequest) (event.Event, error)
}

type EventsHandler struct {
	repo EventsCreator
}

func NewEventsHandler(repo EventsCreator) *EventsHandler {
	return &EventsHandler{repo:  repo}
}


func (e *EventsHandler) CreateEvent(ctx *gin.Context) {
	var req event.CreateEventRequest

	err := ctx.ShouldBindJSON(&req)

	if err != nil {
		ctx.JSON(http.StatusBadRequest,gin.H{
			"error": "Invalid request",
			"message": err.Error(),
		})

		return
	}

	event, err := e.repo.Create(req)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H {
			"error": "internal error",
			"message": "could not create the event",
		})

		return
	}

	ctx.JSON(http.StatusCreated,event)
}