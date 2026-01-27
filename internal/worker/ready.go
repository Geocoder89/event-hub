package worker

import (
	"context"
	"net/http"
	"time"
)

type ReadinessDeps interface {
	Ping(ctx context.Context) error
}

func ReadyHandler(deps ReadinessDeps, isShuttingDown func() bool) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if isShuttingDown() {
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 500 * time.Millisecond)
		defer cancel()

		err := deps.Ping(ctx)

		if err != nil {
			http.Error(w, "db not ready", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	return mux
}