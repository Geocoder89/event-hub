package main

import (
	"context"
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

	// Root context cancelled on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	defer stop()

	// start up the observability logger
	log := observability.NewLogger(cfg.Env)

	pool, err := db.NewPool(cfg.DBURL)

	if err != nil {
		log.Error("db connection failed", "err", err)
		os.Exit(1)
	}

	defer pool.Close()

	// Seed admin user(with timeout)

	seedCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = db.EnsureAdminUser(seedCtx, pool, cfg)

	if err != nil {
		cancel()
		log.Error("failed to seed admin user", "err", err)
		os.Exit(1)
	}
	cancel()

	// set up routers with the log
	router := httpx.NewRouter(log, pool, cfg)

	// server set up
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// start server in the background using an anonymous function

	go func() {
		log.Info("server starting", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	// Block until we get SIGINT/SIGTERM

	<-ctx.Done()

	log.Info("shutdown signal received")

	// Graceful shutdown with timeout

	shutdownContext, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFunc()

	err = srv.Shutdown(shutdownContext)

	if err != nil {
		log.Error("server graceful shutdown failed", "err", err)
		_ = srv.Close() // last resort
	} else {
		log.Info("server stopped gracefully.")
	}
}
