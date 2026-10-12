package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"tabletopper/internal/htmx"
	"tabletopper/internal/hub"
	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"

	"github.com/oklog/ulid/v2"
)

var (
	errNotAMember = errors.New("rooms: not a member of that room")
	errRoomClosed = fmt.Errorf("%w: the room is closed", errNotAMember)
)

func (a *App) roomMember(ctx context.Context, sess session.UserSession, id string) (queries.GetRoomRow, room.Role, error) {
	roomID, err := ulid.Parse(id)
	if err != nil {
		return queries.GetRoomRow{}, "", errNotAMember
	}
	row, err := a.Queries.GetRoom(ctx, roomID)
	if errors.Is(err, sql.ErrNoRows) {
		return queries.GetRoomRow{}, "", errNotAMember
	}
	if err != nil {
		return queries.GetRoomRow{}, "", err
	}
	if row.OwnerID == sess.UserID {
		return row, room.RoleGM, nil
	}
	if sess.RoomID == nil || *sess.RoomID != roomID {
		return queries.GetRoomRow{}, "", errNotAMember
	}
	if row.ClosedAt.Valid {
		return queries.GetRoomRow{}, "", errRoomClosed
	}
	return row, room.RolePlayer, nil
}
func (a *App) member(w http.ResponseWriter, r *http.Request) (queries.GetRoomRow, room.Role, bool) {
	ctx := r.Context()
	row, role, err := a.roomMember(ctx, session.FromContext(ctx), roomParam(r))
	if err != nil {
		if !errors.Is(err, errNotAMember) {
			slog.Error("Failed to load a room", "error", err)
		}
		missing(w, r, "room")
		return queries.GetRoomRow{}, "", false
	}
	return row, role, true
}
func (a *App) liveMember(w http.ResponseWriter, r *http.Request) (queries.GetRoomRow, room.Role, bool) {
	if a.Hub == nil {
		missing(w, r, "room")
		return queries.GetRoomRow{}, "", false
	}
	return a.member(w, r)
}
func (a *App) gm(w http.ResponseWriter, r *http.Request) (queries.GetRoomRow, bool) {
	row, role, ok := a.member(w, r)
	if !ok {
		return queries.GetRoomRow{}, false
	}
	if role != room.RoleGM {
		missing(w, r, "room")
		return queries.GetRoomRow{}, false
	}
	return row, true
}
func (a *App) gmTable(w http.ResponseWriter, r *http.Request) (queries.GetRoomRow, *hub.TableView, bool) {
	row, ok := a.gm(w, r)
	if !ok {
		return queries.GetRoomRow{}, nil, false
	}
	if a.Hub == nil {
		missing(w, r, "room")
		return queries.GetRoomRow{}, nil, false
	}
	view, ok := a.Hub.Table(r.Context(), row.ID)
	if !ok {
		missing(w, r, "room")
		return queries.GetRoomRow{}, nil, false
	}
	return row, view, true
}
func roomParam(r *http.Request) string {
	if id := r.PathValue("id"); id != "" {
		return id
	}
	return r.URL.Query().Get("room")
}
func missing(w http.ResponseWriter, r *http.Request, what string) {
	if r.Method == http.MethodGet {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	htmx.NotFound(w, what)
}
