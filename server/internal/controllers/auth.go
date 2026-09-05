package controllers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"tabletopper/internal/queries"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

// defaultAvatarURL is what a user without a picture of their own gets. It is
// also the column default in the schema; this copy is for the row and the
// session, which are written with an explicit value.
const defaultAvatarURL = "/images/default-avatar.webp"

// Authorize is where Clerk hands a signed-in browser back to us. It verifies
// Clerk's own session cookie, finds or creates our user row, and starts one
// of our sessions.
func (a *App) Authorize(w http.ResponseWriter, r *http.Request) {
	// No Clerk cookie means the browser has not been through Clerk's UI yet,
	// or clerk-js has not run to set it. The sign-in page loads clerk-js and
	// comes straight back here once a Clerk session exists.
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

	// Clerk is the source of truth for the profile; the row only remembers
	// what Clerk said last time, so it fills in where Clerk had nothing.
	sess := session.UserSession{
		Username:        identity.Username,
		ProfileImageURL: identity.ImageURL,
	}

	row, err := a.Queries.GetUserByClerkID(ctx, identity.ClerkID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		slog.Info("New user signed up", "clerkID", identity.ClerkID)
		sess.UserID = ulid.Make()
		if sess.ProfileImageURL == "" {
			sess.ProfileImageURL = defaultAvatarURL
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
		if sess.Username == "" {
			sess.Username = row.Username
		}
		if sess.ProfileImageURL == "" {
			sess.ProfileImageURL = row.ProfileImageURL
		}
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
