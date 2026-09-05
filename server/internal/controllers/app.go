// Package controllers holds the HTTP handlers. Every one is a method on App,
// which carries the dependencies main built once; a handler never reaches
// for a global.
package controllers

import (
	"log/slog"
	"net/http"

	"tabletopper/internal/config"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/storage"

	"github.com/a-h/templ"
	"github.com/clerkinc/clerk-sdk-go/clerk"
)

type App struct {
	Queries  *queries.Queries
	Storage  *storage.Client
	Clerk    clerk.Client
	Sessions *session.Store
	Config   config.Config
}

// render writes a component and logs a failure. Nothing more can be done for
// the client at that point: the status line and part of the body are already
// on the wire.
func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	if err := c.Render(r.Context(), w); err != nil {
		slog.Error("Failed to render", "path", r.URL.Path, "error", err)
	}
}

// redirect is the plain 303 for a browser navigation. An htmx request wants
// htmx.Redirect instead, so fetch() does not follow the hop and swap the
// destination page into the caller's target.
func redirect(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusSeeOther)
}

func redirectToError(w http.ResponseWriter, r *http.Request) {
	redirect(w, r, "/error")
}
