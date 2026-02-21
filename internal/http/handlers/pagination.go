package handlers

import "github.com/gin-gonic/gin"

func BuildCursorPageResponse[T any](limit int, items []T, hasMore bool, nextCursor *string, total *int) gin.H {
	return gin.H{
		"limit":      limit,
		"count":      len(items),
		"items":      items,
		"hasMore":    hasMore,
		"nextCursor": nextCursor,
		"total":      total,
	}
}
