package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"main/internal/controllers"
	db "main/internal/database"
	"main/internal/helpers"
	"main/internal/middleware"
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

	if err := db.Init(); err != nil {
		os.Exit(1)
	}

	cleanupCtx, stopCleanup := context.WithCancel(context.Background())
	defer stopCleanup()
	session.StartCleanup(cleanupCtx, db.Get())

	mux := http.NewServeMux()

	mux.HandleFunc("/", middleware.OptionalSession(func(w http.ResponseWriter, r *http.Request) {
		// NOTE: required because the "/" route is the catch-all
		if r.URL.Path != "/" {
			slog.Warn("404 Not Found", "path", r.URL.Path)
			http.NotFoundHandler().ServeHTTP(w, r)
			return
		}

		pages.Homepage(session.FromContext(r.Context())).Render(r.Context(), w)
	}))

	mux.HandleFunc("/characters", middleware.RequireSession(controllers.CharactersPage))
	mux.HandleFunc("GET /characters/new", middleware.RequireSession(controllers.NewCharacterPage))
	mux.HandleFunc("POST /characters", middleware.RequireSession(controllers.NewCharacterForm))
	mux.HandleFunc("/characters/{id}/edit", middleware.RequireSession(controllers.CharacterPage))
	mux.HandleFunc("POST /characters/{id}", middleware.RequireSession(controllers.EditCharacterForm))
	mux.HandleFunc("DELETE /characters/{id}/delete", middleware.RequireSession(controllers.DeleteCharacter))
	mux.HandleFunc("GET /characters/fragments/info-row", middleware.RequireSession(controllers.InfoRowFragment))

	mux.HandleFunc("/assets", middleware.RequireSession(controllers.AssetsPage))
	mux.HandleFunc("GET /assets/maps", middleware.RequireSession(controllers.MapAssetsPage))
	mux.HandleFunc("POST /assets/maps", middleware.RequireSession(controllers.UploadMap))
	mux.HandleFunc("DELETE /assets/maps/{id}", middleware.RequireSession(controllers.DeleteMap))
	mux.HandleFunc("POST /assets/maps/{id}", middleware.RequireSession(controllers.ReplaceMap))
	mux.HandleFunc("PATCH /assets/maps/{id}/name", middleware.RequireSession(controllers.ReplaceMapName))

	mux.HandleFunc("POST /assets/characters/{id}", middleware.RequireSession(controllers.UploadCharacterAvatar))
	mux.HandleFunc("GET /assets/images/{id}", middleware.RequireSessionOr404(controllers.GetImage))
	mux.HandleFunc("GET /assets/images/{id}/preview", middleware.RequireSessionOr404(controllers.GetImagePreview))

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
		if err := session.Logout(r, w, db.Get()); err != nil {
			helpers.RedirectToError(w, r)
			return
		}
		helpers.Redirect(w, r, "/")
	})

	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("__session")
		if err != nil {
			slog.Error("Failed to get user session from cookie", "error", err)
			helpers.Redirect(w, r, "/sign-in?next=authorize")
			return
		}
		if cookie.Value == "" {
			slog.Error("No token found in cookie")
			helpers.RedirectToError(w, r)
			return
		}

		sessClaims, err := client.VerifyToken(cookie.Value)
		if err != nil {
			slog.Error("Failed to verify token", "error", err)
			helpers.RedirectToError(w, r)
			return
		}
		user, err := client.Users().Read(sessClaims.Claims.Subject)
		if err != nil {
			slog.Error("Failed to read Clerk user", "error", err)
			helpers.RedirectToError(w, r)
			return
		}

		ctx := r.Context()
		q := queries.New(db.Get())

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
						ID:              id,
						Username:        *user.Username,
						ClerkID:         user.ID,
						ProfileImageUrl: user.ProfileImageURL,
					})
					if err != nil {
						slog.Error("Failed to create user", "error", err)
						helpers.RedirectToError(w, r)
						return
					}
				} else {
					err = q.CreateUser(ctx, queries.CreateUserParams{
						ID:       id,
						Username: *user.Username,
						ClerkID:  user.ID,
					})
					if err != nil {
						slog.Error("Failed to create user", "error", err)
						helpers.RedirectToError(w, r)
						return
					}
				}
			} else {
				slog.Error("DB error when querying user by Clerk ID", "error", err)
				helpers.RedirectToError(w, r)
				return
			}
		} else {
			s.UserId = result.ID
			s.ProfileImageURL = result.ProfileImageUrl
			s.Username = result.Username
		}

		if len(*user.Username) > 0 {
			s.Username = *user.Username
		}
		if len(user.ProfileImageURL) > 0 {
			s.ProfileImageURL = user.ProfileImageURL
		}

		err = s.CreateSession(db.Get(), ctx)
		if err != nil {
			slog.Error("Failed to create session", "error", err)
			helpers.RedirectToError(w, r)
			return
		}

		s.SetCookie(w)
		helpers.Redirect(w, r, "/")
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
			stopCleanup()
			_ = db.Close()
			os.Exit(1)
		}
		slog.Info("Shutting down")
		stopCleanup()
		_ = db.Close()
		return
	}

	stopCleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Graceful shutdown failed; forcing closed", "err", err)
		_ = server.Close()
		_ = db.Close()
		os.Exit(1)
	}

	if err := db.Close(); err != nil {
		slog.Error("Failed to close DB pool", "err", err)
	}

	slog.Info("Server shutdown complete")
	os.Exit(0)
}
