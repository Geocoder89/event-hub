package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/geocoder89/eventhub/internal/cache"
	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/domain/event"
	"github.com/geocoder89/eventhub/internal/utils"
	"github.com/gin-gonic/gin"
)

type EventsCreator interface {
	Create(ctx context.Context, req event.CreateEventRequest) (event.Event, error)
	GetByID(ctx context.Context, id string) (event.Event, error)

	// initial offset pagination
	List(ctx context.Context, filter event.ListEventsFilter) ([]event.Event, int, error)

	// NEW: keyset pagination + optional count
	ListCursor(ctx context.Context, filter event.ListEventsFilter, afterStartAt time.Time, afterID string) (items []event.Event, nextCursor *string, hasMore bool, err error)
	Count(ctx context.Context, filter event.ListEventsFilter) (int, error)

	// update and delete events

	Update(ctx context.Context, id string, req event.UpdateEventRequest) (event.Event, error)
	Delete(ctx context.Context, id string) error
}

type EventsHandler struct {
	repo  EventsCreator
	cache *cache.Cache
}

func NewEventsHandler(repo EventsCreator) *EventsHandler {
	return &EventsHandler{repo: repo, cache: nil}
}

func NewEventsHandlerWithCache(repo EventsCreator, c *cache.Cache) *EventsHandler {
	return &EventsHandler{repo: repo, cache: c}
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
		fmt.Println(err)
		RespondInternal(ctx, "Could not create event")
		return
	}

	if e.cache != nil {
		e.cache.Clear()
	}

	ctx.JSON(http.StatusCreated, event)
}

func (h *EventsHandler) ListEvents(ctx *gin.Context) {
	limit := parseIntDefault(ctx.Query("limit"), 20)
	if limit < 1 || limit > 100 {
		RespondBadRequest(ctx, "invalid_query", "limit must be between 1 and 100")
		return
	}

	// filters
	var cityPtr *string
	if city := ctx.Query("city"); city != "" {
		cityPtr = &city
	}

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

	filter := event.ListEventsFilter{
		City:  cityPtr,
		From:  fromPtr,
		To:    toPtr,
		Limit: limit,
	}

	includeTotal := ctx.Query("includeTotal") == "true"
	cursor := ctx.Query("cursor")

	// Option A: first page works with NO cursor
	afterStartAt := time.Unix(0, 0).UTC()
	afterID := "00000000-0000-0000-0000-000000000000" // IMPORTANT: valid UUID

	if cursor != "" {
		cur, err := utils.DecodeEventCursor(cursor)
		if err != nil {
			RespondBadRequest(ctx, "invalid_query", "cursor is invalid")
			return
		}
		afterStartAt = cur.StartAt
		afterID = cur.ID

	}

	cacheable := cursor == "" && !includeTotal && h.cache != nil
	cacheKey := ""

	if cacheable {
		cacheKey = utils.BuildEventsListCacheKey(limit, cityPtr, fromPtr, toPtr)

		v, ok := h.cache.Get(cacheKey)

		if ok {
			slog.Info("events.list.cache_hit", "key", cacheKey)
			ctx.JSON(http.StatusOK, v)
			return
		}
		slog.Info("events.list.cache_miss", "key", cacheKey)
	}

	cctx, cancel := config.WithTimeout(2 * time.Second)
	defer cancel()

	items, next, hasMore, err := h.repo.ListCursor(cctx, filter, afterStartAt, afterID)
	if err != nil {
		RespondInternal(ctx, "Could not list events")
		return
	}

	var total any = nil
	if includeTotal {
		t, err := h.repo.Count(cctx, filter)
		if err != nil {
			RespondInternal(ctx, "Could not count events")
			return
		}
		total = t
	}

	resp := gin.H{
		"limit":      limit,
		"count":      len(items),
		"items":      items,
		"hasMore":    hasMore,
		"nextCursor": next,  // *string -> null when nil
		"total":      total, // null unless includeTotal=true
	}

	if cacheable {
		h.cache.Set(cacheKey, resp)
	}

	ctx.JSON(http.StatusOK, resp)
}

func (h *EventsHandler) GetEventById(c *gin.Context) {
	id := c.Param("id")

	if !utils.IsUUID(id) {
		RespondBadRequest(c, "invalid_id", "id must be a valid UUID")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	slog.Default().InfoContext(ctx, "events.get_by_id", "event_id", id)

	e, err := h.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, event.ErrNotFound) {
			RespondNotFound(c, "Event not found")
			return
		}
		slog.Default().ErrorContext(ctx, "events.get_by_id_failed", "event_id", id, "err", err)
		RespondInternal(c, "Could not fetch event")
		return
	}

	c.JSON(http.StatusOK, e)
}

func (h *EventsHandler) UpdateEvent(ctx *gin.Context) {
	id := ctx.Param("id")

	if !utils.IsUUID(id) {
		RespondBadRequest(ctx, "invalid_id", "id must be a valid UUID")
		return
	}

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

	if h.cache != nil {
		h.cache.Clear()
	}
	ctx.JSON(http.StatusOK, e)
}

func (h *EventsHandler) DeleteEvent(ctx *gin.Context) {
	id := ctx.Param("id")

	if !utils.IsUUID(id) {
		RespondBadRequest(ctx, "invalid_id", "id must be a valid UUID")
		return
	}

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

	if h.cache != nil {
		h.cache.Clear()
	}
	ctx.Status(http.StatusNoContent) //204 empty body.
}
