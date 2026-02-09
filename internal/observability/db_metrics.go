package observability

import (
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func (p *Prom) ObserveDB(op string, fn func() error) error {
	start := time.Now()
	err := fn()

	status := "ok"

	if err != nil {
		status = "error"
		p.DbErrorsTotal.WithLabelValues(op, classifyDBErr(err)).Inc()
	}
	p.DbQueryDuration.WithLabelValues(op, status).Observe(time.Since(start).Seconds())
	return err

}

func classifyDBErr(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return "unique_violation"
		case "40001":
			return "serialization_failure"
		case "40P01":
			return "deadlock"
		case "57014":
			return "query_canceled"
		default:
			return "pg_" + pgErr.Code
		}
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return "timeout"
	case strings.Contains(msg, "connection"):
		return "connection"
	default:
		return "unknown"
	}
}
