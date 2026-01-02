package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func MaxBodyBytes(max int64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, max)

		ctx.Next()
	}
}
