package controllers

import (
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
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, role, err := a.roomMember(ctx, sess, r.PathValue("id"))
	switch {
	case errors.Is(err, errRoomClosed):
		if err := a.Sessions.LeaveRoom(ctx, &sess); err != nil {
			slog.Error("Failed to clear a closed room from a session", "error", err)
		}
		redirect(w, r, "/rooms/join")
		return
	case errors.Is(err, errNotAMember):
		redirect(w, r, "/rooms/join")
		return
	case err != nil:
		slog.Error("Failed to load room", "error", err)
		redirectToError(w, r)
		return
	}
	socket := ""
	if !row.ClosedAt.Valid {
		socket = "/socket/room/" + row.ID.String()
	}
	characterID := roomCharacter(sess, row.ID)
	render(w, r, pages.Room(pages.RoomPageData{
		CharacterID:   idString(characterID),
		CharacterName: a.characterName(ctx, sess.UserID, characterID),
		ID:            row.ID.String(),
		Name:          row.Name,
		Code:          row.Code.String,
		Locked:        row.IsLocked,
		Closed:        row.ClosedAt.Valid,
		Role:          role,
		UserID:        sess.UserID.String(),
		Socket:        socket,
		Version:       a.hubVersion(),
		Debug:         a.Config.Development(),
		FollowTurn:    sess.Prefs.FollowTurn,
		ShowBlood:     sess.Prefs.ShowBlood,
		PingVolume:    sess.Prefs.PingVolume,
		TurnAlert:     sess.Prefs.TurnAlert,
		TurnVolume:    sess.Prefs.TurnVolume,
		TurnNotify:    sess.Prefs.TurnNotify,
		MusicVolume:   sess.Prefs.MusicVolume,
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
	roomID, ok := pathID(w, r, "id", "room")
	if !ok {
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
	renderFragment(w, r, pages.RoomLockItem(pages.RoomPageData{
		ID:     roomID.String(),
		Locked: locked,
		Role:   room.RoleGM,
	}))
}
func (a *App) CloseRoom(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	roomID, ok := pathID(w, r, "id", "room")
	if !ok {
		return
	}
	a.stopRoom(ctx, roomID, sess.UserID)
	err := a.tx(ctx, func(q *queries.Queries) error {
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
	htmx.Toast(w, "The room is closed.")
	htmx.Redirect(w, "/rooms")
}
func (a *App) OpenRoom(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	roomID, ok := pathID(w, r, "id", "room")
	if !ok {
		return
	}
	var matched int64
	err := withRoomCode(func(code string) error {
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
func idString(id *ulid.ULID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
