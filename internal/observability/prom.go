package observability

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

type Prom struct {
	RequestsTotal    *prometheus.CounterVec
	RequestsDuration *prometheus.HistogramVec
	InFlight         *prometheus.GaugeVec
	// DB
	DbQueryDuration *prometheus.HistogramVec
	DbErrorsTotal   *prometheus.CounterVec

	// Jobs(worker)

	JobDuration  *prometheus.HistogramVec
	JobResults   *prometheus.CounterVec
	JobsInFlight prometheus.Gauge
}

func NewProm(reg prometheus.Registerer) *Prom {
	p := &Prom{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "eventhub",
				Name:      "http_requests_total",
				Help:      "Total HTTP requests processed",
			},
			[]string{"method", "route", "status"},
		),
		RequestsDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "eventhub",
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request latency distributions.",
				// Sane initial defaults
				Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
			},
			[]string{"method", "route", "status"},
		),
		InFlight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "eventhub",
				Name:      "http_in_flight_requests",
				Help:      "Current number of in-flight HTTP requests.",
			},
			[]string{"method", "route"},
		),
		DbQueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "eventhub",
				Subsystem: "db",
				Name:      "query_duration_seconds",
				Help:      "DB operation latency (logical op, not raw SQL)",
				Buckets:   []float64{0.005, 0.01, 0.02, 0.05, 0.1, 0.2, 0.35, 0.5, 1, 2, 5},
			},
			[]string{"op", "status"},
		),
		DbErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "eventhub",
				Subsystem: "db",
				Name:      "errors_total",
				Help:      "DB errors by logical op and class.",
			},
			[]string{"op", "class"},
		),

		JobDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "eventhub",
				Subsystem: "jobs",
				Name:      "duration_seconds",
				Help:      "Job execution duration by type and result",
				Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60},
			},
			[]string{"job_type", "result"}, // result=done|retry|failed
		),
		JobResults: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "eventhub",
				Subsystem: "jobs",
				Name:      "results_total",
				Help:      "Job outcomes by type and result.",
			},
			[]string{"job_type", "result"}, // result=done|retry|failed
		),
		JobsInFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "eventhub",
				Subsystem: "jobs",
				Name:      "in_flight",
				Help:      "Current number of executing jobs across workers(per process)",
			},
		),
	}
	reg.MustRegister(p.RequestsTotal, p.RequestsDuration, p.InFlight, p.DbQueryDuration, p.DbErrorsTotal, p.JobDuration, p.JobResults, p.JobsInFlight)

	return p
}

func (p *Prom) GinHandleMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()

		// route template is only available after routing; best effort:
		route := ctx.FullPath()

		if route == "" {
			route = "unmatched"
		}

		method := ctx.Request.Method
		p.InFlight.WithLabelValues(method, route).Inc()
		defer p.InFlight.WithLabelValues(method, route).Dec()
		ctx.Next()

		status := strconv.Itoa(ctx.Writer.Status())
		secs := time.Since(start).Seconds()

		p.RequestsTotal.WithLabelValues(method, route, status).Inc()
		p.RequestsDuration.WithLabelValues(method, route, status).Observe(secs)
	}
}
