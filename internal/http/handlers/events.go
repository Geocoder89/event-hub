package handlers

import (
	"net/http"

	"github.com/geocoder89/eventhub/internal/domain/event"
	"github.com/geocoder89/eventhub/internal/repo/memory"
	"github.com/gin-gonic/gin"
)




type EventsCreator interface {
	Create(req event.CreateEventRequest) (event.Event, error)
	GetByID(id string) (event.Event, error)
	List()([]event.Event,error)
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

func(h *EventsHandler) ListEvents(ctx *gin.Context) {
events, err :=	h.repo.List()

if err != nil {
	ctx.JSON(http.StatusInternalServerError, gin.H{
		"error": "internal error",
		"message": "could not list events",
	})

	return 
}

ctx.JSON(http.StatusOK,gin.H{
	"items": events,
	"count": len(events),
})
}

func(h *EventsHandler) GetById(ctx *gin.Context){

	id := ctx.Param("id")
	e, err := h.repo.GetByID(id)

	if err != nil {
		if err == memory.ErrNotFound {
			ctx.JSON(http.StatusNotFound,gin.H{
				"error": "not_found",
				"message": "event not found",
			})
			return 
		}
		ctx.JSON(http.StatusInternalServerError,gin.H{
			"error": "internal_error",
			"message": "could not fetch event",
		})
	}

	ctx.JSON(http.StatusOK, e)


}