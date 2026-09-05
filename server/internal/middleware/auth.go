package middleware

import (
	"log/slog"

	"net/http"
	db "tabletopper/internal/database"
	"tabletopper/internal/helpers"
	"tabletopper/internal/session"
)

// RequireSession loads the user session and stashes it on the request context,
// bouncing to the sign-in page when there isn't one.
func RequireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := session.GetUserSessionFromCookie(r, db.Get())
		if err != nil {
			redirectToSignIn(w, r)
			return
		}
		refresh(&s, r, w)
		next(w, withSession(r, s))
	}
}

// RequireSessionOr404 is RequireSession for the asset proxy routes, where a
// redirect would render as a broken image rather than a navigation. It skips
// the refresh: these are sub-resources of a page request that just did one.
func RequireSessionOr404(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := session.GetUserSessionFromCookie(r, db.Get())
		if err != nil {
			http.NotFoundHandler().ServeHTTP(w, r)
			return
		}
		next(w, withSession(r, s))
	}
}

// OptionalSession loads the user session when there is one and continues either
// way, for pages that render both a logged-in and a logged-out state.
func OptionalSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := session.GetUserSessionFromCookie(r, db.Get())
		if err != nil {
			next(w, r)
			return
		}
		refresh(&s, r, w)
		next(w, withSession(r, s))
	}
}

// refresh slides the session's expiry forward. It runs before the handler so
// the Set-Cookie header lands ahead of any response body, and a failure is
// logged rather than propagated: a session that is still valid should not be
// rejected because the write failed.
func refresh(s *session.UserSession, r *http.Request, w http.ResponseWriter) {
	if err := s.Refresh(r, w, db.Get()); err != nil {
		slog.Error("Failed to refresh session", "error", err)
	}
}

func withSession(r *http.Request, s session.UserSession) *http.Request {
	return r.WithContext(session.NewContext(r.Context(), s))
}

// redirectToSignIn sends HTMX requests an HX-Redirect, since a 303 would be
// swapped into the page as sign-in markup instead of navigating.
func redirectToSignIn(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		helpers.HTMXRedirect(w, "/sign-in")
		return
	}
	helpers.RedirectToSignIn(w, r)
}
