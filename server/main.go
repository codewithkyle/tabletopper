package main

import (
	"context"
	"errors"
	"log/slog"
	db "main/internal/database"
	"main/internal/session"
	"main/templ/pages"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// NOTE: required because the "/" route is the catch-all
		if r.URL.Path != "/" {
			slog.Warn("404 Not Found", "path", r.URL.Path)
			http.NotFoundHandler().ServeHTTP(w, r)
			return
		}

		db, err := db.Connect()
		if err != nil {
			http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
		}

		session, err := session.GetUserSessionFromCookie(r, db)
		if err != nil {
			if err != http.ErrNoCookie {
				http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
			}
		}

		pages.Homepage(session).Render(r.Context(), w)
	})

	mux.HandleFunc("/tos", func(w http.ResponseWriter, r *http.Request) {
		pages.TOS().Render(r.Context(), w)
	})

	mux.HandleFunc("/privacy", func(w http.ResponseWriter, r *http.Request) {
		pages.PrivacyPolicy().Render(r.Context(), w)
	})

	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		pages.ServerError().Render(r.Context(), w)
	})

	// NOTE: static files
	mux.Handle(
		"/css/",
		http.StripPrefix(
			"/css/",
			http.FileServer(http.Dir("./public/css")),
		),
	)
	mux.Handle(
		"/js/",
		http.StripPrefix(
			"/js/",
			http.FileServer(http.Dir("./public/js")),
		),
	)
	mux.Handle(
		"/static/",
		http.StripPrefix(
			"/static/",
			http.FileServer(http.Dir("./public/static")),
		),
	)
	mux.Handle(
		"/audio/",
		http.StripPrefix(
			"/audio/",
			http.FileServer(http.Dir("./public/audio")),
		),
	)
	mux.Handle(
		"/images/",
		http.StripPrefix(
			"/images/",
			http.FileServer(http.Dir("./public/images")),
		),
	)

	server := &http.Server{
		Addr:         ":3000",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("Listening on :3000")
		errCh <- server.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case _ = <-sigCh:
		slog.Info("Shutting down")
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server error", "err", err)
			os.Exit(1)
		}
		slog.Info("Shutting down")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Graceful shutdown failed; forcing closed", "err", err)
		_ = server.Close()
		os.Exit(1)
	}

	slog.Info("Server shutdown complete")
	os.Exit(0)
}
