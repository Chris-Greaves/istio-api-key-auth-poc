// Command server runs the API key authenticator service.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/app"
	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg)
	if err != nil {
		logger.Error("failed to start application", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := application.Close(); err != nil {
			logger.Error("error closing application", "error", err)
		}
	}()

	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: application.Handler(),
	}

	shutdownComplete := make(chan struct{})
	go func() {
		defer close(shutdownComplete)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("error during server shutdown", "error", err)
		}
	}()

	logger.Info("starting server", "addr", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}

	// ListenAndServe returns as soon as Shutdown closes the listener, which is
	// before Shutdown has finished draining in-flight requests. Wait for the
	// shutdown goroutine so the deferred Close() above doesn't close the
	// database out from under a request that's still being handled.
	<-shutdownComplete
}
