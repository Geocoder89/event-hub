package middlewares

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequireJSON() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			ct := c.GetHeader("Content-Type")
			// allow "application/json; charset=utf-8"
			if ct == "" || !strings.HasPrefix(strings.ToLower(ct), "application/json") {
				c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{
					"error": gin.H{
						"code":    "unsupported_media_type",
						"message": "Content-Type must be application/json",
					},
				})
				return
			}
		}
		c.Next()
	}
}
