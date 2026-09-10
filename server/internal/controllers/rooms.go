package controllers

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/go-sql-driver/mysql"
	"github.com/oklog/ulid/v2"
)

const (
	// roomNameLimit mirrors rooms.name, which is VARCHAR(128). The dialog
	// carries the same number as a maxlength; this is what refuses a client
	// that ignored it, and refusing here is what keeps MySQL from truncating a
	// name rather than rejecting it.
	roomNameLimit = pages.RoomNameLimit

	// roomCodeTries is how many fresh codes an insert will try before giving
	// up. A collision means the code is already held by another OPEN room, and
	// with about 920,000 codes against a handful open at once, one collision is
	// already improbable and five in a row is not something that happens -- so
	// this is a bound on a retry loop rather than a capacity plan. Without it a
	// bug that made every code identical would spin forever holding a
	// connection.
	roomCodeTries = 5

	// mysqlDuplicateEntry is ER_DUP_ENTRY. It is the one error the insert is
	// allowed to swallow, because it means "that code is taken" and the answer
	// to that is another code -- every other error is a failure the GM has to
	// be told about.
	mysqlDuplicateEntry = 1062
)

// errRoomNotFound is what a room statement inside a transaction returns when it
// matched no rows, so the transaction rolls back rather than committing the
// statements around it. It never reaches the client: each caller turns it into
// the same 404 an unmatched statement outside a transaction produces.
//
// IT IS THE WHOLE OF THE OWNERSHIP CHECK ON THE UNSCOPED STATEMENTS.
// ClearRoomSessions takes a room and no owner -- it has to, since it is written
// against every session in a room and not against the caller's -- so the thing
// that keeps somebody from emptying a table they do not own is that the
// owner-scoped statement runs first in the same transaction and this error
// rolls the other one back.
var errRoomNotFound = errors.New("rooms: no such room for this owner")

// RoomsPage is the GM's own rooms, newest first. It is the roster's shape for
// tables: a list of cards and a dialog above them that creates one from a name.
func (a *App) RoomsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	rows, err := a.Queries.ListRooms(ctx, sess.UserID)
	if err != nil {
		slog.Error("Failed to load rooms", "error", err)
		redirectToError(w, r)
		return
	}

	summaries := make([]pages.RoomSummary, 0, len(rows))
	for _, row := range rows {
		summaries = append(summaries, pages.RoomSummary{
			ID:     row.ID.String(),
			Name:   row.Name,
			Code:   row.Code.String,
			Locked: row.IsLocked,
			Closed: row.ClosedAt.Valid,
		})
	}

	render(w, r, pages.Rooms(pages.RoomsPageData{Rooms: summaries}))
}

// NewRoomFragment serves the content of the new-room dialog: a heading, one
// field and a button. Like the character and monster dialogs it reads no
// database and nothing off the session, because the form it returns is the same
// for every user -- and like them it stays behind auth.Fragment all the same,
// since an unauthenticated route here would be surface for no reason.
func (a *App) NewRoomFragment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.NewRoomFragment())
}

// NewRoomForm creates a room from a name and sends the GM to it. The name is
// the only thing collected: everything else a room has is answered by the
// schema or minted here, and the table itself is set up on the page afterwards.
//
// THE CODE IS MINTED HERE AND NOT BY THE DATABASE, because it is random rather
// than sequential and MySQL has nothing that produces one. That means the
// insert can collide with a code another open room already holds, so it is
// retried with a fresh code -- see withRoomCode.
//
// The reply on success is a redirect with no body, so nothing lands back in the
// dialog; the navigation takes it away. The toast still arrives, on the page
// after this one, because toast.js parks a message in sessionStorage when the
// same response also carries HX-Redirect.
func (a *App) NewRoomForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	if err := r.ParseForm(); err != nil {
		rejectNewRoom(w, r, "The submitted form data could not be read.")
		return
	}

	name := strings.TrimSpace(r.PostFormValue("name"))
	switch {
	case name == "":
		rejectNewRoom(w, r, "Name is required.")
		return
	case len([]rune(name)) > roomNameLimit:
		rejectNewRoom(w, r, "Name must be 128 characters or fewer.")
		return
	}

	id := ulid.Make()
	err := withRoomCode(func(code string) error {
		return a.Queries.CreateRoom(ctx, queries.CreateRoomParams{
			ID:      id,
			OwnerID: sess.UserID,
			Name:    name,
			Code:    sql.NullString{String: code, Valid: true},
		})
	})
	if err != nil {
		slog.Error("Failed to create room", "error", err)
		htmx.ServerError(w)
		return
	}

	htmx.Toast(w, name+" is ready.")
	htmx.Redirect(w, "/rooms/"+id.String())
}

// DeleteRoom removes a room and turns everybody at it out, in one transaction.
//
// THE OWNER-SCOPED STATEMENT RUNS FIRST AND THAT IS THE ORDER, not a
// preference. ClearRoomSessions names a room and no owner, so on its own it
// would empty a table belonging to somebody else; running the delete ahead of
// it means a caller who owns nothing gets errRoomNotFound and the transaction
// rolls back before the second statement has done anything.
//
// It answers 200 with an empty body and not 204 -- noSwap lists 204, and a
// status in that list overrides the hx-swap="delete" on the button, which would
// leave the card on screen after the room was gone.
func (a *App) DeleteRoom(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	roomID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "room")
		return
	}

	err = a.tx(ctx, func(q *queries.Queries) error {
		result, err := q.DeleteRoom(ctx, queries.DeleteRoomParams{ID: roomID, OwnerID: sess.UserID})
		if err != nil {
			return err
		}
		if rows, err := result.RowsAffected(); err != nil {
			return err
		} else if rows == 0 {
			return errRoomNotFound
		}

		_, err = q.ClearRoomSessions(ctx, &roomID)
		return err
	})
	if errors.Is(err, errRoomNotFound) {
		htmx.NotFound(w, "room")
		return
	}
	if err != nil {
		slog.Error("Failed to delete room", "error", err)
		htmx.ServerError(w)
		return
	}

	// THE LIVE ROOM IS ENDED AS WELL, as CloseRoom ends it. Without this the
	// goroutine plays on over a row that is gone: commands apply, events
	// broadcast, and its save matches zero rows and reports success, so the
	// GM sees the room gone from the list while everybody in it goes on
	// playing for as long as the last socket stays open. Close tells them,
	// drops them, and unloads it; the one save it makes on the way out lands
	// on nothing, which is fine.
	if a.Hub != nil {
		a.Hub.Close(ctx, roomID)
	}

	w.WriteHeader(http.StatusOK)
}

// rejectNewRoom answers with the dialog's error block under a 422, which is the
// one code the form has an hx-status route for -- every other 4xx is in the
// noSwap list and would leave the dialog showing nothing new. The form is left
// alone, so the name the GM typed is still in the field when the message
// appears above it.
func rejectNewRoom(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnprocessableEntity)
	render(w, r, pages.PanelFormErrors(pages.NewRoomPanel, []string{message}))
}

// withRoomCode runs fn with a freshly minted code and hands back whatever fn
// says, except that a collision on the unique index over rooms.code is retried
// with another code rather than reported.
//
// The alternative was to read the table first and pick a code nothing holds,
// which is the same query with a race in it: two rooms created in the same
// moment would both find their code free. The insert is what settles it, so the
// insert is what asks.
func withRoomCode(fn func(code string) error) error {
	var err error

	for range roomCodeTries {
		if err = fn(room.NewCode()); !isDuplicateCode(err) {
			return err
		}
	}

	return fmt.Errorf("rooms: %d codes in a row were already taken: %w", roomCodeTries, err)
}

// isDuplicateCode reports whether err is MySQL refusing a row for a unique key
// it already holds. Only rooms.code is unique on this table besides the primary
// key, and the primary key is a fresh ULID, so this can only ever be the code.
func isDuplicateCode(err error) bool {
	var mysqlErr *mysql.MySQLError

	return errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlDuplicateEntry
}
