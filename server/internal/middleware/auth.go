// Package middleware wraps handlers with the session lookup. Each wrapper
// answers the same question, "who is asking?", and differs only in what it
// does when the answer is nobody.
package middleware

import (
	"log/slog"
	"net/http"

	"tabletopper/internal/htmx"
	"tabletopper/internal/session"
)

// Auth holds the session store the wrappers read from. main builds one and
// registers routes through it.
type Auth struct {
	Sessions *session.Store
}

// RequireSession loads the user session and stashes it on the request context,
// bouncing to the sign-in page when there isn't one.
func (m Auth) RequireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := m.Sessions.FromRequest(r)
		if err != nil {
			redirectToSignIn(w, r)
			return
		}
		noStore(w)
		m.refresh(w, r, &s)
		next(w, withSession(r, s))
	}
}

// RequireSessionOr404 is RequireSession for the asset proxy routes, where a
// redirect would render as a broken image rather than a navigation. It skips
// the refresh: these are sub-resources of a page request that just did one.
//
// IT ALSO SKIPS noStore, and that is the point of it being a third wrapper.
// Every route behind this one sets a cache policy of its own -- `private,
// no-cache` for a picture that is replaced at its URL, `private, immutable` for
// a tile and a journal image that never are -- and those are what make a map
// pannable and a sheet's avatar not refetched on every page. A no-store written
// here would be replaced by each of them anyway; leaving it out is what says
// the omission is deliberate rather than missed.
func (m Auth) RequireSessionOr404(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := m.Sessions.FromRequest(r)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		next(w, withSession(r, s))
	}
}

// OptionalSession loads the user session when there is one and continues either
// way, for pages that render both a logged-in and a logged-out state.
func (m Auth) OptionalSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := m.Sessions.FromRequest(r)
		if err != nil {
			next(w, r)
			return
		}
		noStore(w)
		m.refresh(w, r, &s)
		next(w, withSession(r, s))
	}
}

// noStore keeps a page rendered for one signed-in reader out of every cache
// between here and their screen.
//
// THE CASE IT IS FOR IS THE SHARED MACHINE. A character sheet or the account
// page held in the browser's back-forward cache, or by a forward proxy, is
// still there after the logout that was supposed to end it -- the session row
// is gone and the cookie is cleared, and neither of those is consulted to
// replay a document a cache already has. The fragments got this from the
// start; the pages they are swapped into did not.
//
// no-store rather than no-cache, because no-cache permits storing it and only
// requires revalidation, and there is nothing to revalidate against once the
// session that authorised it has ended.
func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

// refresh slides the session's expiry forward. It runs before the handler so
// the Set-Cookie header lands ahead of any response body, and a failure is
// logged rather than propagated: a session that is still valid should not be
// rejected because the write failed.
func (m Auth) refresh(w http.ResponseWriter, r *http.Request, s *session.UserSession) {
	if err := m.Sessions.Refresh(r.Context(), w, s); err != nil {
		slog.Error("Failed to refresh session", "error", err)
	}
}

func withSession(r *http.Request, s session.UserSession) *http.Request {
	return r.WithContext(session.NewContext(r.Context(), s))
}

// redirectToSignIn sends htmx requests an HX-Redirect, since a 303 would be
// swapped into the page as sign-in markup instead of navigating.
func redirectToSignIn(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		htmx.Redirect(w, "/sign-in")
		return
	}
	http.Redirect(w, r, "/sign-in", http.StatusSeeOther)
}
