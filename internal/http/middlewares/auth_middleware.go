package middlewares

import (
	"net/http"
	"strings"

	"github.com/geocoder89/eventhub/internal/auth"
	"github.com/gin-gonic/gin"
)

// Keep this small interface so tests can fake it easily.
type TokenVerifier interface {
	VerifyAccessToken(token string) (*auth.Claims, error)
}

type AuthMiddleware struct {
	jwt TokenVerifier
}

func NewAuthMiddleware(jwt TokenVerifier) *AuthMiddleware {
	return &AuthMiddleware{jwt: jwt}
}

const (
	ctxUserIDKey = "auth.userID"
	ctxEmailKey  = "auth.email"
	ctxRoleKey   = "auth.role"
)

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "unauthorized",
					"message": "Missing or invalid Authorization header",
				},
			})
			return
		}

		raw := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "unauthorized",
					"message": "Missing or invalid access token",
				},
			})
			return
		}

		claims, err := m.jwt.VerifyAccessToken(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "unauthorized",
					"message": "Invalid or expired access token",
				},
			})
			return
		}

		// Stash useful bits of identity on the context
		c.Set(ctxUserIDKey, claims.UserID)
		c.Set(ctxEmailKey, claims.Email)
		c.Set(ctxRoleKey, claims.Role)

		c.Next()
	}
}

// Optional helpers so handlers don’t need to know the magic keys.

func UserIDFromContext(c *gin.Context) (string, bool) {
	v, ok := c.Get(ctxUserIDKey)
	if !ok {
		return "", false
	}
	id, ok := v.(string)
	return id, ok
}

func RoleFromContext(c *gin.Context) (string, bool) {
	v, ok := c.Get(ctxRoleKey)
	if !ok {
		return "", false
	}
	role, ok := v.(string)
	return role, ok
}
