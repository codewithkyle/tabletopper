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

	// Clerk is the source of truth for the picture; the row only remembers what
	// Clerk said last time, so it fills in where Clerk had nothing.
	//
	// THE NAME GOES THE OTHER WAY. users.username is the account's own display
	// name, which the settings dialog can change, so Clerk seeds it at sign-up
	// and never touches it again -- a login that copied Clerk's value over the
	// row would silently undo every rename on the reader's next visit.
	sess := session.UserSession{ProfileImageURL: identity.ImageURL}

	row, err := a.Queries.GetUserByClerkID(ctx, identity.ClerkID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		slog.Info("New user signed up", "clerkID", identity.ClerkID)
		sess.UserID = ulid.Make()
		sess.Username = identity.Username
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
		sess.Username = row.Username
		if sess.ProfileImageURL == "" {
			sess.ProfileImageURL = row.ProfileImageURL
		}

		// AN ACCOUNT WITH NO NAME PREDATES THE FALLBACK. Clerk's username is
		// optional and an OAuth sign-up need not have one, so before
		// clerkauth resolved a name out of the profile and the email, a Google
		// sign-up wrote an empty string here and kept it. Seeding it now is
		// what a rename would have done, and it happens once: the row is not
		// empty the next time this runs.
		if sess.Username == "" && identity.Username != "" {
			sess.Username = identity.Username

			err := a.Queries.SetUsername(ctx, queries.SetUsernameParams{
				ID:       sess.UserID,
				Username: sess.Username,
			})
			if err != nil {
				// Not fatal. The session already carries the name, so this
				// login reads correctly and the next one tries again.
				slog.Warn("Failed to seed a display name", "error", err)
			}
		}
	}

	// The row this login replaces goes first. It is logged rather than fatal:
	// a session that could not be ended is a stale row the sweep will collect,
	// and refusing the login over it would lock somebody out of their account
	// because of a row they are done with.
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
