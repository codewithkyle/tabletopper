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
// run is main with a return value, so every exit path is a returned error
// and the deferred cleanup runs on all of them.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	// ctx ends on SIGINT or SIGTERM. Everything long-lived hangs off it: the
	// three sweepers and the map tiler stop, and the server drains.
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
	// THE ROOMS. One goroutine per live room, loaded on the first join and
	// unloaded ten minutes after the last person leaves. It is constructed
	// here rather than lazily because Shutdown below has to reach it: a deploy
	// in the middle of Saturday's game has to write every room back before the
	// process goes, and a hub nothing had built yet would have nothing to
	// write.
	rooms := hub.New(q, hub.Options{})
	app := &controllers.App{
		DB:       pool,
		Queries:  q,
		Storage:  store,
		Clerk:    clerkauth.New(cfg.ClerkAPIKey),
		Sessions: sessions,
		Config:   cfg,
		Hub:      rooms,
		// Ten tries a minute per share. Generous for somebody mistyping a
		// password out of a chat message, and a rate at which a six-character
		// guess never finishes.
		ShareAttempts: share.NewAttempts(10, time.Minute),
		// Ten tries a minute per user against the room codes. A player
		// mistyping a code read out over voice chat gets several goes; a
		// script walking the 920,000 codes at this rate would need about
		// seventeen years to cover them.
		RoomJoinAttempts: share.NewAttempts(10, time.Minute),
	}
	auth := middleware.Auth{Sessions: sessions}
	// THE UPLOAD ROUTES LIFT THESE PER REQUEST rather than the numbers here
	// being relaxed for everything. ReadTimeout covers the whole body and not
	// just the headers, and WriteTimeout runs from when the headers were read
	// rather than from when the handler answers, so at these values a large
	// upload cannot be received and, if it were, could not be replied to. A
	// handler that expects one extends both of its own deadlines through an
	// http.ResponseController, which overrides what these established; every
	// other route keeps five seconds, which is what stops a connection being
	// held open on a body nobody is sending.
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
		// ListenAndServe only returns before Shutdown is called when it
		// failed, so ErrServerClosed cannot arrive here.
		return fmt.Errorf("server: %w", err)
	}
	// THE ROOMS ARE SAVED ON EVERY PATH OUT OF HERE, and on a budget of their
	// own. This is what the whole snapshot design exists for -- a deploy in
	// the middle of Saturday's game -- so it must not depend on the HTTP drain
	// having gone well or having left any of a shared deadline unspent. A drain
	// held up by a long upload used to return an error before the rooms were
	// reached, and a drain that succeeded slowly handed them a context that
	// was already done.
	//
	// AFTER THE SERVER HAS DRAINED, not before, which the defer guarantees by
	// running last. Shutdown closes every socket with going-away so the
	// browsers reconnect to the next process instead of showing an error, and
	// a room closed while the listener was still open would be reopened by the
	// reconnect that followed.
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
// The two halves of the shutdown budget. They are separate rather than one
// shared ten seconds so that a slow HTTP drain cannot spend the time the rooms
// need to write themselves back; see the deferred block in run.
const (
	httpShutdownBudget  = 5 * time.Second
	roomsShutdownBudget = 5 * time.Second
)
