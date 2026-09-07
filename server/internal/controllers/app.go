// Package controllers holds the HTTP handlers. Every one is a method on App,
// which carries the dependencies main built once; a handler never reaches
// for a global.
package controllers

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"

	"tabletopper/internal/clerkauth"
	"tabletopper/internal/config"
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

	// DB is the pool Queries was built over, and it is here for exactly one
	// reason: the handful of writes that are several statements and have to
	// land together. See tx.
	DB *sql.DB

	// ShareAttempts is the counter in front of the one unauthenticated
	// password check in the app. It is on App rather than a package-level in
	// share because the limit and the window are a deployment's decision and
	// because a test wants its own, and it is required rather than optional:
	// main builds it, and UnlockShare asks it before bcrypt runs.
	ShareAttempts *share.Attempts

	// RoomJoinAttempts is the same counter in front of the room join, and it is
	// a second instance rather than a second use of the first: a share token and
	// a room code are different namespaces, and one window shared between them
	// would let a flood of bad codes lock a reader out of a link.
	//
	// THE KEY IS THE USER ID, WHERE THE SHARE COUNTER'S IS THE TOKEN. A share
	// request is unauthenticated, so the only client identity available there is
	// an address Cloudflare wrote into a header -- which is why that one keys by
	// the thing being attacked instead. A join is behind RequireSession, so the
	// user id is an identity the client cannot reset by opening a new tab or
	// changing networks, and keying by it means one person guessing codes locks
	// only themselves out rather than locking a room somebody is trying to join.
	RoomJoinAttempts *share.Attempts
}

// tx runs fn over one transaction. fn gets a Queries bound to it; a returned
// error rolls the whole thing back and nil commits.
//
// MYSQL IS TRANSACTIONAL AND THIS SCHEMA USED NOT TO ACT LIKE IT. The purges
// and the monster import are each several statements against several tables,
// and until this existed each one wrote them in sequence and carried a
// compensating delete to undo what had already landed if a later statement
// failed. That works, and it is more code than a transaction, and it has to be
// extended by hand every time a table is added -- which the room tables are
// about to do. A rollback that the database performs cannot forget a table.
//
// WHAT STAYS OUTSIDE IS R2. An object is not a row and cannot be rolled back
// with one, so every purge still deletes objects first and rows second, in the
// order it always did: an object deleted without its row is a row the reader
// can delete again, and a row deleted without its object is an orphan nothing
// will ever find. Only the row half becomes atomic, which is the half that has
// several steps.
//
// The Rollback error is dropped on purpose. It is reached only because fn
// already failed, that failure is what the caller is told about, and a rollback
// that itself failed has nothing left to do -- the connection is returned to
// the pool and the transaction dies with it.
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
