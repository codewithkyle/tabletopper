package controllers
import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"
	"github.com/oklog/ulid/v2"
)
func (a *App) RoomPage(w http.ResponseWriter, r *http.Request) {
	row, role, ok := a.loadRoomMember(w, r)
	if !ok {
		return
	}
	socket := ""
	if !row.ClosedAt.Valid {
		socket = "/socket/room/" + row.ID.String()
	}
	sess := session.FromContext(r.Context())
	render(w, r, pages.Room(pages.RoomPageData{
		ID:         row.ID.String(),
		Name:       row.Name,
		Code:       row.Code.String,
		Locked:     row.IsLocked,
		Closed:     row.ClosedAt.Valid,
		Role:       role,
		UserID:     sess.UserID.String(),
		Socket:     socket,
		Version:    a.hubVersion(),
		Debug:      a.Config.Development(),
		FollowTurn: sess.Prefs.FollowTurn,
		ShowBlood:  sess.Prefs.ShowBlood,
		PingVolume: sess.Prefs.PingVolume,
	}))
}
func (a *App) LockRoom(w http.ResponseWriter, r *http.Request) {
	a.setRoomLocked(w, r, true)
}
func (a *App) UnlockRoom(w http.ResponseWriter, r *http.Request) {
	a.setRoomLocked(w, r, false)
}
func (a *App) setRoomLocked(w http.ResponseWriter, r *http.Request, locked bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	roomID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "room")
		return
	}
	result, err := a.Queries.SetRoomLocked(ctx, queries.SetRoomLockedParams{
		IsLocked: locked,
		ID:       roomID,
		OwnerID:  sess.UserID,
	})
	if err != nil {
		slog.Error("Failed to set room lock", "error", err)
		htmx.ServerError(w)
		return
	}
	rows, err := result.RowsAffected()
	if err != nil {
		slog.Error("Failed to set room lock", "error", err)
		htmx.ServerError(w)
		return
	}
	if rows == 0 {
		htmx.NotFound(w, "room")
		return
	}
	a.notify(roomID, &room.RoomSetLocked{Locked: locked})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomLockItem(pages.RoomPageData{
		ID:     roomID.String(),
		Locked: locked,
		Role:   room.RoleGM,
	}))
}
func (a *App) CloseRoom(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	roomID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "room")
		return
	}
	err = a.tx(ctx, func(q *queries.Queries) error {
		result, err := q.CloseRoom(ctx, queries.CloseRoomParams{ID: roomID, OwnerID: sess.UserID})
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
		slog.Error("Failed to close room", "error", err)
		htmx.ServerError(w)
		return
	}
	if a.Hub != nil {
		a.Hub.Close(ctx, roomID)
	}
	htmx.Toast(w, "The room is closed.")
	htmx.Redirect(w, "/rooms")
}
func (a *App) OpenRoom(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	roomID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "room")
		return
	}
	var matched int64
	err = withRoomCode(func(code string) error {
		result, err := a.Queries.OpenRoom(ctx, queries.OpenRoomParams{
			Code:    sql.NullString{String: code, Valid: true},
			ID:      roomID,
			OwnerID: sess.UserID,
		})
		if err != nil {
			return err
		}
		matched, err = result.RowsAffected()
		return err
	})
	if err != nil {
		slog.Error("Failed to reopen room", "error", err)
		htmx.ServerError(w)
		return
	}
	if matched == 0 {
		htmx.NotFound(w, "room")
		return
	}
	htmx.Toast(w, "The room is open again.")
	htmx.Redirect(w, "/rooms/"+roomID.String())
}
func (a *App) LeaveRoom(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	roomID, err := ulid.Parse(r.PathValue("id"))
	if err != nil || sess.RoomID == nil || *sess.RoomID != roomID {
		htmx.NotFound(w, "room")
		return
	}
	if err := a.Sessions.LeaveRoom(ctx, &sess); err != nil {
		slog.Error("Failed to leave room", "error", err)
		htmx.ServerError(w)
		return
	}
	a.notify(roomID, &room.PlayerLeave{ID: sess.UserID})
	htmx.Toast(w, "You left the room.")
	htmx.Redirect(w, "/")
}
func (a *App) notify(roomID ulid.ULID, cmd room.Command) {
	if a.Hub == nil {
		return
	}
	a.Hub.Notify(roomID, cmd)
}
func (a *App) hubVersion() string {
	if a.Hub == nil {
		return ""
	}
	return a.Hub.Version()
}
func (a *App) loadRoomMember(w http.ResponseWriter, r *http.Request) (queries.GetRoomRow, room.Role, bool) {
	row, role, err := a.roomMember(r.Context(), session.FromContext(r.Context()), r.PathValue("id"))
	switch {
	case errors.Is(err, errNotAMember):
		redirect(w, r, "/rooms/join")
		return queries.GetRoomRow{}, "", false
	case err != nil:
		slog.Error("Failed to load room", "error", err)
		redirectToError(w, r)
		return queries.GetRoomRow{}, "", false
	}
	return row, role, true
}
var errNotAMember = errors.New("rooms: not a member of that room")
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
		if err := a.Sessions.LeaveRoom(ctx, &sess); err != nil {
			slog.Error("Failed to clear a closed room from a session", "error", err)
		}
		return queries.GetRoomRow{}, "", errNotAMember
	}
	return row, room.RolePlayer, nil
}
