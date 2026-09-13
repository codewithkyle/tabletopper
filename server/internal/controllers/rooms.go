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
	roomNameLimit       = pages.RoomNameLimit
	roomCodeTries       = 5
	mysqlDuplicateEntry = 1062
)

var errRoomNotFound = errors.New("rooms: no such room for this owner")

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
func (a *App) NewRoomFragment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.NewRoomFragment())
}
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
func (a *App) DeleteRoom(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	roomID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "room")
		return
	}
	a.closeScene(ctx, roomID, sess.UserID)
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
	if a.Hub != nil {
		a.Hub.Close(ctx, roomID)
	}
	w.WriteHeader(http.StatusOK)
}
func rejectNewRoom(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnprocessableEntity)
	render(w, r, pages.PanelFormErrors(pages.NewRoomPanel, []string{message}))
}
func withRoomCode(fn func(code string) error) error {
	var err error
	for range roomCodeTries {
		if err = fn(room.NewCode()); !isDuplicateCode(err) {
			return err
		}
	}
	return fmt.Errorf("rooms: %d codes in a row were already taken: %w", roomCodeTries, err)
}
func isDuplicateCode(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlDuplicateEntry
}
