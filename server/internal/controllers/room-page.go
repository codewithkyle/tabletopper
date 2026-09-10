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

// RoomPage is the shell the tabletop mounts on: a menu bar across the top, the
// table filling everything under it, and the tool pill floating over it. From
// this phase it is also live -- the page carries what the socket module needs
// to connect and nothing else.
//
// THE SOCKET PATH IS EMPTY FOR A CLOSED ROOM, and that is how the client is
// told not to connect. A closed room cannot be loaded by the hub, so a browser
// that tried would be refused and would keep retrying on its backoff forever;
// leaving the attribute off is one condition in one place instead.
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

	// THE ROW IS WRITTEN FIRST AND THE ROOM IS TOLD SECOND, always, for every
	// fact the rooms row owns. Going the other way would put two writers on one
	// truth and make "is this room locked" a question with two answers; this
	// way the room is a mirror, and a room that is not running has nothing to
	// mirror because its next load reads the column.
	a.notify(roomID, &room.RoomSetLocked{Locked: locked})

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

	// Everybody still at the table is told, dropped, and the room saves one
	// last time on its way out -- so reopening it comes back to the pawns where
	// they were left.
	if a.Hub != nil {
		a.Hub.Close(ctx, roomID)
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

	// Leaving is not disconnecting: the row goes, so the pawns this person
	// owned stop being theirs to move and their line leaves the tracker.
	a.notify(roomID, &room.PlayerLeave{ID: sess.UserID})

	htmx.Toast(w, "You left the room.")
	htmx.Redirect(w, "/")
}

// notify mirrors a change into a live room and does nothing when there is no
// hub -- which is the routes test and several handler tests, and never a
// running server.
func (a *App) notify(roomID ulid.ULID, cmd room.Command) {
	if a.Hub == nil {
		return
	}

	a.Hub.Notify(roomID, cmd)
}

// hubVersion is the build string the page puts on its bundle URL, so a client
// that reloads after a deploy cannot be served last week's script out of the
// one-hour static cache.
func (a *App) hubVersion() string {
	if a.Hub == nil {
		return ""
	}

	return a.Hub.Version()
}

// loadRoomMember is the access rule for the room page, written on its own
// because it is three questions and the page asks all three.
//
// EVERY REFUSAL LANDS ON THE JOIN PAGE, and none of them says why. This is a
// page request, so the answer is a 303 a browser follows -- and a full
// navigation has no channel for a message, which is why there is no toast here
// to explain the closed room.
//
// ok is false when this function has already written the response.
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

// errNotAMember is every way a request can be turned away from a room: the id
// is not a ULID, there is no such row, the caller is neither the owner nor
// somebody whose session points at it, or the room is closed and they are not
// the GM. They are one error because they get one answer -- the caller is not
// told which, since telling somebody "that room exists but is not yours" is a
// way to enumerate rooms.
var errNotAMember = errors.New("rooms: not a member of that room")

// roomMember is the access rule itself, with no ResponseWriter in it, because
// three callers want three different refusals: the page redirects, the socket
// and the fragment answer 404.
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
