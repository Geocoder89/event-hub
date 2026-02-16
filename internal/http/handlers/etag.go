package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RespondJSONWithETag(ctx *gin.Context, status int, payload interface{}) {
	etag, err := buildETag(payload)
	if err != nil {
		ctx.JSON(status, payload)
		return
	}

	ctx.Header("ETag", etag)

	if ifNoneMatchMatches(ctx.GetHeader("If-None-Match"), etag) {
		ctx.Status(http.StatusNotModified)
		return
	}

	ctx.JSON(status, payload)
}

func buildETag(payload interface{}) (string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(b)

	return `"` + hex.EncodeToString(sum[:]) + `"`, nil
}

func ifNoneMatchMatches(headerValue, currentETag string) bool {
	if strings.TrimSpace(headerValue) == "" || strings.TrimSpace(currentETag) == "" {
		return false
	}

	if strings.TrimSpace(headerValue) == "*" {
		return true
	}

	current := normalizeETag(currentETag)

	for _, part := range strings.Split(headerValue, ",") {
		if normalizeETag(part) == current {
			return true
		}
	}

	return false
}

func normalizeETag(raw string) string {
	v := strings.TrimSpace(raw)

	// RFC allows weak validators like W/"abc".
	if strings.HasPrefix(v, "W/") {
		v = strings.TrimSpace(strings.TrimPrefix(v, "W/"))
	}

	return v
}
