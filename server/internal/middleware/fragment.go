package middleware

import (
	"log/slog"

	"net/http"
	db "tabletopper/internal/database"
	"tabletopper/internal/helpers"
	"tabletopper/internal/session"
)

// Fragment is the contract for everything mounted under /fragment/: a GET whose
// body is partial HTML, destined for a swap into a page that is already open.
//
// It is RequireSession with the guesswork taken out. RequireSession has to sniff
// the HX-Request header to choose between a 303 and an HX-Redirect, because a
// 303 to /sign-in would be followed by fetch() and swapped into the page as
// sign-in markup. Down here the prefix has already answered that question --
// nothing reaches a /fragment/ route except an htmx swap -- so the redirect is
// unconditional and there is no header left to get wrong.
//
// The two headers are the other half of the reason the prefix exists. A fragment
// is a piece of a page and is worthless on its own, so it must never be indexed
// as a document or replayed from a cache into a page it was not rendered for.
// Setting them here is what keeps that from being a per-handler chore.
func Fragment(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Robots-Tag", "noindex")

		s, err := session.GetUserSessionFromCookie(r, db.Get())
		if err != nil {
			helpers.HTMXRedirect(w, "/sign-in")
			return
		}

		// Refreshed, unlike RequireSessionOr404: a fragment is fetched when the
		// user acts, which can be a long time after the page load that would
		// have slid the expiry forward.
		refresh(&s, r, w)
		next(w, withSession(r, s))
	}
}

// FragmentNotFound answers a /fragment/ path that matched no route. It is
// registered instead of letting the catch-all on "/" take these, so the log line
// says a fragment was missed rather than a page, and so the body stays empty:
// the noSwap config in base.templ covers 4xx, which means htmx leaves the target
// untouched and the content modal falls through to its error state.
func FragmentNotFound(w http.ResponseWriter, r *http.Request) {
	slog.Warn("404 Not Found (fragment)", "path", r.URL.Path)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex")
	w.WriteHeader(http.StatusNotFound)
}
