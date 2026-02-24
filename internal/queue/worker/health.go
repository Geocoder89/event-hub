package worker

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (w *Worker) HealthHandler(reg *prometheus.Registry) http.Handler {
	r := gin.New()

	r.Use(gin.Recovery())

	// liveness: process is up

	r.GET("/healthz", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"ok": true,
		})
	})

	// readiness: worker is able to claim + process
	// keeping it simple: exposing an internal flag which can flip when shutting down
	r.GET("/readyz", func(c *gin.Context) {
		w.readyMu.RLock()
		ready := w.ready
		w.readyMu.RUnlock()

		if !ready {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
			return
		}

		if w.readinessCheck != nil {
			checkCtx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
			defer cancel()

			if err := w.readinessCheck(checkCtx); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"status": "dependency_unavailable",
					"error":  err.Error(),
				})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// Prometheus
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	return r
}
