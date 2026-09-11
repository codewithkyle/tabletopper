


package controllers

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"

	"tabletopper/internal/clerkauth"
	"tabletopper/internal/config"
	"tabletopper/internal/hub"
	"tabletopper/internal/queries"
	"tabletopper/internal/session"
	"tabletopper/internal/share"
	"tabletopper/internal/storage"

	"github.com/a-h/templ"
)

type App struct {
	Queries  *queries.Queries
	Storage  *storage.Client
	Clerk    *clerkauth.Client
	Sessions *session.Store
	Config   config.Config

	
	
	
	
	
	
	
	
	
	Hub *hub.Hub

	
	
	
	DB *sql.DB

	
	
	
	
	
	ShareAttempts *share.Attempts

	
	
	
	
	
	
	
	
	
	
	
	
	RoomJoinAttempts *share.Attempts
}























func (a *App) tx(ctx context.Context, fn func(q *queries.Queries) error) error {
	tx, err := a.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}

	if err := fn(a.Queries.WithTx(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}




func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	if err := c.Render(r.Context(), w); err != nil {
		slog.Error("Failed to render", "path", r.URL.Path, "error", err)
	}
}




func redirect(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusSeeOther)
}

func redirectToError(w http.ResponseWriter, r *http.Request) {
	redirect(w, r, "/error")
}
