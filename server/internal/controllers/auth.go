package controllers
import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"github.com/oklog/ulid/v2"
)
func (a *App) Authorize(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("__session")
	if err != nil || cookie.Value == "" {
		redirect(w, r, "/sign-in")
		return
	}
	ctx := r.Context()
	identity, err := a.Clerk.Authenticate(ctx, cookie.Value)
	if err != nil {
		slog.Error("Failed to authenticate with Clerk", "error", err)
		redirectToError(w, r)
		return
	}
	sess := session.UserSession{ProfileImageURL: identity.ImageURL}
	row, err := a.Queries.GetUserByClerkID(ctx, identity.ClerkID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		slog.Info("New user signed up", "clerkID", identity.ClerkID)
		sess.UserID = ulid.Make()
		sess.Username = identity.Username
		if sess.ProfileImageURL == "" {
			sess.ProfileImageURL = room.DefaultAvatar
		}
		err := a.Queries.CreateUser(ctx, queries.CreateUserParams{
			ID:              sess.UserID,
			Username:        sess.Username,
			ClerkID:         identity.ClerkID,
			ProfileImageURL: sess.ProfileImageURL,
		})
		if err != nil {
			slog.Error("Failed to create user", "error", err)
			redirectToError(w, r)
			return
		}
	case err != nil:
		slog.Error("Failed to query user by Clerk ID", "error", err)
		redirectToError(w, r)
		return
	default:
		sess.UserID = row.ID
		sess.Username = row.Username
		if sess.ProfileImageURL == "" {
			sess.ProfileImageURL = row.ProfileImageURL
		}
		if sess.Username == "" && identity.Username != "" {
			sess.Username = identity.Username
			err := a.Queries.SetUsername(ctx, queries.SetUsernameParams{
				ID:       sess.UserID,
				Username: sess.Username,
			})
			if err != nil {
				slog.Warn("Failed to seed a display name", "error", err)
			}
		}
	}
	if err := a.Sessions.EndCurrent(r); err != nil {
		slog.Warn("Failed to end the previous session", "error", err)
	}
	if err := a.Sessions.Create(ctx, w, &sess); err != nil {
		slog.Error("Failed to create session", "error", err)
		redirectToError(w, r)
		return
	}
	redirect(w, r, "/")
}
func (a *App) Logout(w http.ResponseWriter, r *http.Request) {
	if err := a.Sessions.Logout(w, r); err != nil {
		slog.Error("Failed to log out", "error", err)
		redirectToError(w, r)
		return
	}
	redirect(w, r, "/")
}
