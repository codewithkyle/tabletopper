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

// defaultAvatarURL is what a user without a Clerk profile image gets. It is
// also the column default in the schema; this copy is for the session, which
// is built before the row is read back.
const defaultAvatarURL = "/images/default-avatar.webp"

// Authorize is where Clerk hands a signed-in browser back to us. It verifies
// Clerk's own session cookie, finds or creates our user row, and starts one
// of our sessions.
func (a *App) Authorize(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("__session")
	if err != nil {
		slog.Error("Failed to get Clerk session cookie", "error", err)
		redirect(w, r, "/sign-in?next=authorize")
		return
	}
	if cookie.Value == "" {
		slog.Error("Clerk session cookie is empty")
		redirectToError(w, r)
		return
	}

	claims, err := a.Clerk.VerifyToken(cookie.Value)
	if err != nil {
		slog.Error("Failed to verify Clerk token", "error", err)
		redirectToError(w, r)
		return
	}
	user, err := a.Clerk.Users().Read(claims.Claims.Subject)
	if err != nil {
		slog.Error("Failed to read Clerk user", "error", err)
		redirectToError(w, r)
		return
	}

	// NOTE: Clerk models the username as a pointer because a user who signed
	// up through an OAuth provider may not have one yet
	username := ""
	if user.Username != nil {
		username = *user.Username
	}

	ctx := r.Context()
	sess := session.UserSession{ProfileImageURL: defaultAvatarURL}

	row, err := a.Queries.GetUserByClerkID(ctx, user.ID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		slog.Info("Someone signed up!", "clerkID", user.ID)
		id := ulid.Make()
		sess.UserID = id
		if user.ProfileImageURL != "" {
			err = a.Queries.CreateUserWithAvatar(ctx, queries.CreateUserWithAvatarParams{
				ID:              id,
				Username:        username,
				ClerkID:         user.ID,
				ProfileImageURL: user.ProfileImageURL,
			})
		} else {
			err = a.Queries.CreateUser(ctx, queries.CreateUserParams{
				ID:       id,
				Username: username,
				ClerkID:  user.ID,
			})
		}
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
		sess.ProfileImageURL = row.ProfileImageURL
		sess.Username = row.Username
	}

	// Clerk is the source of truth for the profile, so what it says now wins
	// over what the row remembers.
	if username != "" {
		sess.Username = username
	}
	if user.ProfileImageURL != "" {
		sess.ProfileImageURL = user.ProfileImageURL
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
