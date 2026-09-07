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

// RoomPage is the shell every later phase of the tabletop mounts on: a menu bar
// across the top, the table filling everything under it, and the tool pill
// floating over it. Nothing on it is live yet.
func (a *App) RoomPage(w http.ResponseWriter, r *http.Request) {
	row, role, ok := a.loadRoomMember(w, r)
	if !ok {
		return
	}

	render(w, r, pages.Room(pages.RoomPageData{
		ID:     row.ID.String(),
		Name:   row.Name,
		Code:   row.Code.String,
		Locked: row.IsLocked,
		Closed: row.ClosedAt.Valid,
		Role:   role,
	}))
}

// LockRoom shuts the door on a room that is already running: whoever is in it
// stays, and nobody else gets in until it is unlocked.
func (a *App) LockRoom(w http.ResponseWriter, r *http.Request) {
	a.setRoomLocked(w, r, true)
}

// UnlockRoom is the mirror of LockRoom.
func (a *App) UnlockRoom(w http.ResponseWriter, r *http.Request) {
	a.setRoomLocked(w, r, false)
}

// setRoomLocked is both lock routes, because they differ by one boolean.
//
// IT ANSWERS WITH THE MENU ITEM IT JUST CHANGED, which is the mutation case the
// fragment rules name -- the alternative is a POST that returns nothing
// followed by a GET to fetch what it did. Lock and Unlock are one line in the
// Room menu and two routes, so the reply is that line in its new state, built
// by the same function that built the one the page first drew.
//
// IT NEEDS NO READ. The statement is owner-scoped and carries the new value, so
// a row it matched is a room this caller owns whose lock is now what was asked
// for; there is nothing left to go and find out.
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomLockItem(pages.RoomPageData{
		ID:     roomID.String(),
		Locked: locked,
		Role:   room.RoleGM,
	}))
}

// CloseRoom ends a session at the table: the code goes back into circulation
// and everybody who was in the room is turned out of it, in one transaction.
//
// THE ROW IS KEPT AND THAT IS THE POINT OF CLOSING RATHER THAN DELETING. The
// room comes back with a new code and, once the transport phase lands, with the
// pawns where they were left -- which is what makes a room worth keeping
// between Saturdays.
//
// The owner-scoped statement runs first, so a caller who owns nothing rolls
// back before ClearRoomSessions -- which names a room and no owner -- has
// touched anything. See errRoomNotFound.
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

	htmx.Toast(w, "The room is closed.")
	htmx.Redirect(w, "/rooms")
}

// OpenRoom brings a closed room back with a new code. The old one is not
// restored: it went back into circulation when the room closed and may belong
// to somebody else's table by now.
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

// LeaveRoom takes one player out of the room they are in. It is the player's
// half of the pair whose GM half is CloseRoom, and it touches only their own
// session row.
//
// THE PATH HAS TO NAME THE ROOM THEY ARE ACTUALLY IN. It is the session that
// says which room that is, so the id here settles nothing -- but a POST naming
// another room is a stale page or a mistake, and answering it by quietly
// clearing whatever room they happened to be in would take somebody out of a
// game because a tab was old.
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

	htmx.Toast(w, "You left the room.")
	htmx.Redirect(w, "/")
}

// loadRoomMember is the access rule for the room page, written on its own
// because it is three questions and the page asks all three.
//
// THE ROLE IS DERIVED AND NOT STORED. rooms.owner_id says who the GM is, and
// sessions.room_id says who else is in the room; a role column would be a
// second copy of the first fact, and there is no room in the design for a
// membership that disagrees with ownership.
//
// A CLOSED ROOM IS STILL THE GM'S. They are the one who has to reopen it, so
// the page renders for them with the Room menu offering Reopen. For a player a
// closed room is over: their session is cleared here rather than left pointing
// at a room they cannot rejoin, because the alternative is a homepage that goes
// on offering "return to your table" for a table that is gone.
//
// EVERY REFUSAL LANDS ON THE JOIN PAGE, and none of them says why. This is a
// page request, so the answer is a 303 a browser follows -- and a full
// navigation has no channel for a message, which is why there is no toast here
// to explain the closed room. When the player list and the other room fragments
// arrive they will want the htmx-shaped half of this, and that is when the
// explanation gets somewhere to go.
//
// ok is false when this function has already written the response.
func (a *App) loadRoomMember(w http.ResponseWriter, r *http.Request) (queries.GetRoomRow, room.Role, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	roomID, err := ulid.Parse(r.PathValue("id"))
	if err != nil {
		redirect(w, r, "/rooms/join")
		return queries.GetRoomRow{}, "", false
	}

	row, err := a.Queries.GetRoom(ctx, roomID)
	if errors.Is(err, sql.ErrNoRows) {
		redirect(w, r, "/rooms/join")
		return queries.GetRoomRow{}, "", false
	}
	if err != nil {
		slog.Error("Failed to load room", "error", err)
		redirectToError(w, r)
		return queries.GetRoomRow{}, "", false
	}

	if row.OwnerID == sess.UserID {
		return row, room.RoleGM, true
	}

	if sess.RoomID == nil || *sess.RoomID != roomID {
		redirect(w, r, "/rooms/join")
		return queries.GetRoomRow{}, "", false
	}

	if row.ClosedAt.Valid {
		if err := a.Sessions.LeaveRoom(ctx, &sess); err != nil {
			slog.Error("Failed to clear a closed room from a session", "error", err)
		}
		redirect(w, r, "/rooms/join")
		return queries.GetRoomRow{}, "", false
	}

	return row, room.RolePlayer, true
}
