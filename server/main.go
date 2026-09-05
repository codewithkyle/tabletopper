package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tabletopper/internal/clerkauth"
	"tabletopper/internal/config"
	"tabletopper/internal/controllers"
	"tabletopper/internal/database"
	"tabletopper/internal/middleware"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/storage"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Fatal", "error", err)
		os.Exit(1)
	}
}

// run is main with a return value, so every exit path is a returned error
// and the deferred cleanup runs on all of them.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// ctx ends on SIGINT or SIGTERM. Everything long-lived hangs off it: the
	// session sweeper stops, and the server drains.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, cfg.DSN)
	if err != nil {
		return err
	}
	defer func() {
		if err := pool.Close(); err != nil {
			slog.Error("Failed to close DB pool", "error", err)
		}
	}()

	store, err := storage.New(ctx, storage.Config{
		AccountID:       cfg.R2AccountID,
		AccessKeyID:     cfg.R2AccessKeyID,
		SecretAccessKey: cfg.R2SecretAccessKey,
		Bucket:          cfg.R2Bucket,
	})
	if err != nil {
		return err
	}

	q := queries.New(pool)
	sessions := session.NewStore(q, !cfg.Development())
	sessions.StartCleanup(ctx)

	app := &controllers.App{
		Queries:  q,
		Storage:  store,
		Clerk:    clerkauth.New(cfg.ClerkAPIKey),
		Sessions: sessions,
		Config:   cfg,
	}
	auth := middleware.Auth{Sessions: sessions}

	server := &http.Server{
		Addr:         cfg.Addr,
		Handler:      routes(app, auth),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("Listening", "addr", cfg.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		slog.Info("Shutting down")
	case err := <-errCh:
		// ListenAndServe only returns before Shutdown is called when it
		// failed, so ErrServerClosed cannot arrive here.
		return fmt.Errorf("server: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server: %w", err)
	}

	slog.Info("Server shutdown complete")
	return nil
}
