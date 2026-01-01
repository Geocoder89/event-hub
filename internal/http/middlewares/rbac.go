package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (m *AuthMiddleware) RequireRole(required string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := RoleFromContext(c)

		if !ok || role == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "unauthorized",
					"message": "Missing identity context",
				},
			})
			return
		}
		if role != required {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":    "forbidden",
					"message": "Admin role required",
				},
			})
			return
		}
		c.Next()
	}
}
