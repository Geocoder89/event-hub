package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/db"
	httpx "github.com/geocoder89/eventhub/internal/http"
	"github.com/geocoder89/eventhub/internal/observability"
	"github.com/joho/godotenv"
)

func main() {
	// Load the config set up
	_ = godotenv.Load()
	cfg := config.Load()

	// start up the observability logger
	log := observability.NewLogger(cfg.Env)

	pool, err := db.NewPool(cfg.DBURL)

	if err != nil {
		log.Error("db connection failed", "err", err)
		os.Exit(1)
	}

	defer pool.Close()

	// set up routers with the log
	router := httpx.NewRouter(log,pool)

	// server set up
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// start server using a concurrent go-routine driven anonymous function.

	go func() {
		log.Info("Server starting", "port", cfg.Port, "env", cfg.Env)
		err := srv.ListenAndServe()

		if err != nil && err != http.ErrServerClosed {
			log.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()


	// Graceful shutdown

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("server shutting down")

	shutdownCh := make(chan struct{})

	go func() {
		defer close(shutdownCh)

		ctxTimeOut := 10 * time.Second

		ctx,cancel :=config.WithTimeout(ctxTimeOut)

		defer cancel()

		err := srv.Shutdown(ctx)

		if err != nil {
			log.Error("graceful shutdown failed", "err", err)

			return 
		}
	}()

	select {
	case <-shutdownCh:
		log.Info("shutdown complete")

	case <-time.After(12 * time.Second):
		log.Error("shutdown timed out")
	}
}
