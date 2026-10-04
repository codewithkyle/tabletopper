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
	"tabletopper/internal/hub"
	"tabletopper/internal/middleware"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/share"
	"tabletopper/internal/storage"
	"tabletopper/internal/sweep"
	"tabletopper/internal/tiling"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
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
	sweep.JournalImages(ctx, q, store)
	sweep.ExpiredShares(ctx, q)
	sweep.MusicUploads(ctx, q, store)
	tiling.Maps(ctx, q, store)
	rooms := hub.New(q, hub.Options{})
	purger := sweep.DeletedAccounts(ctx, q, store, rooms)
	app := &controllers.App{
		DB:               pool,
		Queries:          q,
		Storage:          store,
		Clerk:            clerkauth.New(cfg.ClerkAPIKey),
		Sessions:         sessions,
		Config:           cfg,
		Hub:              rooms,
		ShareAttempts:    share.NewAttempts(10, time.Minute),
		RoomJoinAttempts: share.NewAttempts(10, time.Minute),
		Purger:           purger,
	}
	auth := middleware.Auth{Sessions: sessions}
	server := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler(app, auth),
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
		return fmt.Errorf("server: %w", err)
	}
	defer func() {
		roomsCtx, cancel := context.WithTimeout(context.Background(), roomsShutdownBudget)
		defer cancel()
		rooms.Shutdown(roomsCtx)
		slog.Info("Server shutdown complete")
	}()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownBudget)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server: %w", err)
	}
	return nil
}

const (
	httpShutdownBudget  = 5 * time.Second
	roomsShutdownBudget = 5 * time.Second
)
