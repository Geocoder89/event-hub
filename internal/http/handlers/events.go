package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/domain/event"
	"github.com/gin-gonic/gin"
)

type EventsCreator interface {
	Create(ctx context.Context, req event.CreateEventRequest) (event.Event, error)
	GetByID(ctx context.Context, id string) (event.Event, error)
	List(ctx context.Context, filter event.ListEventsFilter) ([]event.Event, int, error)

	// update and delete events

	Update(ctx context.Context, id string, req event.UpdateEventRequest) (event.Event, error)
	Delete(ctx context.Context, id string) error
}

type EventsHandler struct {
	repo EventsCreator
}

func NewEventsHandler(repo EventsCreator) *EventsHandler {
	return &EventsHandler{repo: repo}
}

// function to make sure, what is returned is a number for the limit query

func parseIntDefault(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}

	return n
}

func (e *EventsHandler) CreateEvent(ctx *gin.Context) {
	var req event.CreateEventRequest

	if !BindJSON(ctx, &req) {
		return
	}

	cctx, cancel := config.WithTimeout(2 * time.Second)

	defer cancel()

	event, err := e.repo.Create(cctx, req)

	if err != nil {
		RespondInternal(ctx, "Could not create event")
		return
	}

	ctx.JSON(http.StatusCreated, event)
}

func (h *EventsHandler) ListEvents(ctx *gin.Context) {
	page := parseIntDefault(ctx.Query("page"), 1)
	limit := parseIntDefault(ctx.Query("limit"), 20)

	if page < 1 {
		RespondBadRequest(ctx, "invalid_query", "page must be >= 1")
		return
	}

	if limit < 1 || limit > 100 {
		RespondBadRequest(ctx, "invalid_query", "limit must be between 1 and 100")
		return
	}

	offset := (page - 1) * limit

	// New filters

	// city filter (optional)
	var cityPtr *string
	if city := ctx.Query("city"); city != "" {
		cityPtr = &city
	}

	// date filters (optional)
	var fromPtr, toPtr *time.Time

	if fromStr := ctx.Query("from"); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			RespondBadRequest(ctx, "invalid_query", "from must be RFC3339 datetime")
			return
		}
		fromPtr = &t
	}

	if toStr := ctx.Query("to"); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			RespondBadRequest(ctx, "invalid_query", "to must be RFC3339 datetime")
			return
		}
		toPtr = &t
	}

	// final generated filter

	filter := event.ListEventsFilter{
		City:   cityPtr,
		From:   fromPtr,
		To:     toPtr,
		Limit:  limit,
		Offset: offset,
	}
	// optional timeout

	cctx, cancel := config.WithTimeout(2 * time.Second)

	defer cancel()

	items, total, err := h.repo.List(cctx, filter)
	// events, err := h.repo.List()

	if err != nil {
		RespondInternal(ctx, "Could not list events")

		fmt.Println(err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"page":  page,
		"limit": limit,
		"count": len(items),
		"total": total,
		"items": items,
	})
}

func (h *EventsHandler) GetEventById(ctx *gin.Context) {

	id := ctx.Param("id")

	cctx, cancel := config.WithTimeout(2 * time.Second)

	defer cancel()
	e, err := h.repo.GetByID(cctx, id)

	if err != nil {
		if err == event.ErrNotFound {
			RespondNotFound(ctx, "Event not found")
			return
		}
		RespondInternal(ctx, "Could not fetch event")
		return
	}

	ctx.JSON(http.StatusOK, e)

}

func (h *EventsHandler) UpdateEvent(ctx *gin.Context) {
	id := ctx.Param("id")

	var req event.UpdateEventRequest

	if !BindJSON(ctx, &req) {
		return
	}

	cctx, cancel := config.WithTimeout(2 * time.Second)

	defer cancel()

	e, err := h.repo.Update(cctx, id, req)

	// checks if the error type is not found, returns a 404
	if err != nil {
		fmt.Println(err)
		if errors.Is(err, event.ErrNotFound) {
			RespondNotFound(ctx, "Event not found")
			return
		}

		// any other error, returns a 500
		RespondInternal(ctx, "Could not update event")
		return

	}
	ctx.JSON(http.StatusOK, e)
}

func (h *EventsHandler) DeleteEvent(ctx *gin.Context) {
	id := ctx.Param("id")

	cctx, cancel := config.WithTimeout(2 * time.Second)

	defer cancel()

	err := h.repo.Delete(cctx, id)

	// checks if the error type is not found, returns a 404
	if err != nil {
		if errors.Is(err, event.ErrNotFound) {
			RespondNotFound(ctx, "Event not found")
			return
		}

		// any other error, returns a 500
		RespondInternal(ctx, "Could not delete event")
		return

	}

	ctx.Status(http.StatusNoContent) //204 empty body.
}
