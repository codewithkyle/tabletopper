package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"main/internal/controllers"
	db "main/internal/database"
	"main/internal/queries"
	"main/internal/session"
	"main/templ/pages"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clerkinc/clerk-sdk-go/clerk"
	"github.com/oklog/ulid/v2"
)

func main() {
	client, err := clerk.NewClient(os.Getenv("CLERK_API_KEY"))
	if err != nil {
		slog.Error("failed to create Clerk client", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// NOTE: required because the "/" route is the catch-all
		if r.URL.Path != "/" {
			slog.Warn("404 Not Found", "path", r.URL.Path)
			http.NotFoundHandler().ServeHTTP(w, r)
			return
		}

		ctx := context.Background()

		db, err := db.Connect()
		if err != nil {
			http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
			return
		}

		session, err := session.GetUserSessionFromCookie(r, db, ctx)
		if err != nil {
			if !errors.Is(err, http.ErrNoCookie) {
				http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
				return
			}
		}

		pages.Homepage(session).Render(r.Context(), w)
	})

	mux.HandleFunc("/characters", controllers.CharactersPage)
	mux.HandleFunc("GET /characters/new", controllers.NewCharacterPage)
	mux.HandleFunc("POST /characters", controllers.NewCharacterForm)
	mux.HandleFunc("/characters/{id}/edit", controllers.CharacterPage)
	mux.HandleFunc("POST /characters/{id}", controllers.EditCharacterForm)
	mux.HandleFunc("DELETE /characters/{id}/delete", controllers.DeleteCharacter)

	mux.HandleFunc("/tos", func(w http.ResponseWriter, r *http.Request) {
		pages.TOS().Render(r.Context(), w)
	})

	mux.HandleFunc("/privacy", func(w http.ResponseWriter, r *http.Request) {
		pages.PrivacyPolicy().Render(r.Context(), w)
	})

	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		pages.ServerError().Render(r.Context(), w)
	})

	mux.HandleFunc("/sign-in", func(w http.ResponseWriter, r *http.Request) {
		pages.SignIn().Render(r.Context(), w)
	})
	mux.HandleFunc("/sign-up", func(w http.ResponseWriter, r *http.Request) {
		pages.SignUp().Render(r.Context(), w)
	})

	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		db, err := db.Connect()
		if err != nil {
			http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
			return
		}
		err = session.Logout(r, w, db, ctx)
		if err != nil {
			http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("__session")
		if err != nil {
			slog.Error("Failed to get user session from cookie", "error", err)
			http.Redirect(w, r, "/sign-in?next=authorize", http.StatusTemporaryRedirect)
			return
		}
		if cookie.Value == "" {
			slog.Error("No token found in cookie")
			http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
			return
		}

		sessClaims, err := client.VerifyToken(cookie.Value)
		if err != nil {
			slog.Error("Failed to verify token", "error", err)
			http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
			return
		}
		user, err := client.Users().Read(sessClaims.Claims.Subject)
		if err != nil {
			slog.Error("Failed to read Clerk user", "error", err)
			http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
			return
		}

		db, err := db.Connect()
		if err != nil {
			http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
			return
		}
		ctx := context.Background()
		q := queries.New(db)

		s := session.New()
		s.ProfileImageURL = "/images/default-avatar.webp"
		result, err := q.GetUserByClerkID(ctx, user.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				slog.Info("Someone signed up!", "clerkId", user.ID)
				id := ulid.Make()
				s.UserId = id
				if len(user.ProfileImageURL) > 0 {
					err = q.CreateUserWithAvatar(ctx, queries.CreateUserWithAvatarParams{
						ID:              id[:],
						Username:        *user.Username,
						ClerkID:         user.ID,
						ProfileImageUrl: user.ProfileImageURL,
					})
					if err != nil {
						slog.Error("Failed to create user", "error", err)
						http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
						return
					}
				} else {
					err = q.CreateUser(ctx, queries.CreateUserParams{
						ID:       id[:],
						Username: *user.Username,
						ClerkID:  user.ID,
					})
					if err != nil {
						slog.Error("Failed to create user", "error", err)
						http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
						return
					}
				}
			} else {
				slog.Error("DB error when querying user by Clerk ID", "error", err)
				http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
				return
			}
		} else {
			slog.Info("Found existing user")
			s.UserId = ulid.ULID(result.ID)
			s.ProfileImageURL = result.ProfileImageUrl
			s.Username = result.Username
		}

		if len(*user.Username) > 0 {
			s.Username = *user.Username
		}
		if len(user.ProfileImageURL) > 0 {
			s.ProfileImageURL = user.ProfileImageURL
		}

		err = s.CreateSession(db, ctx)
		if err != nil {
			slog.Error("Failed to create session", "error", err)
			http.Redirect(w, r, "/error", http.StatusTemporaryRedirect)
			return
		}

		s.SetCookie(w)
		slog.Info("New session started", "name", s.Username)

		http.Redirect(w, r, "/", http.StatusSeeOther)
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
