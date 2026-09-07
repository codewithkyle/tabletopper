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

// roomSource says where in the request the room's id is written, because the
// page takes it from the path and a fragment takes it from the query string --
// a fragment is not the room's URL, it is a piece of a page about the room, the
// same distinction the share and stat-block dialogs make.
type roomSource int

const (
	roomFromPath roomSource = iota
	roomFromQuery
)

// RoomPage is the shell every later phase of the tabletop mounts on: the room's
// name, the GM's controls, an empty table region and a side panel. Nothing on
// it is live yet.
func (a *App) RoomPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	row, role, ok := a.loadRoomMember(w, r, roomFromPath)
	if !ok {
		return
	}

	members, err := a.roomMembers(ctx, row.ID)
	if err != nil {
		slog.Error("Failed to load room members", "error", err)
		redirectToError(w, r)
		return
	}

	render(w, r, pages.Room(pages.RoomPageData{
		ID:      row.ID.String(),
		Name:    row.Name,
		Code:    row.Code.String,
		Locked:  row.IsLocked,
		Closed:  row.ClosedAt.Valid,
		Role:    role,
		Members: members,
	}))
}

// RoomMembersFragment is the panel on its own, for the refresh beside its
// heading. It is the same component the page renders, filled in by the same
// function, which is what the /fragment/ prefix promises.
//
// A ROOM THAT IS NOT YOURS IS AN EMPTY 404 RATHER THAN AN ALERT, and so is a
// `room` that will not parse: both came off the page's own markup, so a request
// carrying either is not somebody who lost their table and has nothing to be
// told. loadRoomMember writes that answer.
func (a *App) RoomMembersFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	row, _, ok := a.loadRoomMember(w, r, roomFromQuery)
	if !ok {
		return
	}

	members, err := a.roomMembers(ctx, row.ID)
	if err != nil {
		slog.Error("Failed to load room members", "error", err)
		htmx.ServerError(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomMembersFragment(members))
}

// LockRoom shuts the door on a room that is already running: the members stay,
// and nobody else gets in until it is unlocked.
func (a *App) LockRoom(w http.ResponseWriter, r *http.Request) {
	a.setRoomLocked(w, r, true)
}

// UnlockRoom is the mirror of LockRoom.
func (a *App) UnlockRoom(w http.ResponseWriter, r *http.Request) {
	a.setRoomLocked(w, r, false)
}

// setRoomLocked is both lock routes, because they differ by one boolean.
//
// IT ANSWERS WITH THE CONTROL IT JUST CHANGED, which is the mutation case the
// fragment rules name -- the alternative is a POST that returns nothing
// followed by a GET to fetch what it did. The control it renders is the same
// component the page drew, so the two cannot drift.
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
	render(w, r, pages.RoomLockControl(pages.RoomPageData{
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

// loadRoomMember is the access rule for the room page and every fragment of it,
// written once because it is three questions and every caller asks all three.
//
// THE ROLE IS DERIVED AND NOT STORED. rooms.owner_id says who the GM is, and
// sessions.room_id says who else is in the room; a role column would be a
// second copy of the first fact, and there is no room in the design for a
// membership that disagrees with ownership.
//
// A CLOSED ROOM IS STILL THE GM'S. They are the one who has to reopen it, so
// the page renders for them with the closed state and the Reopen button. For a
// player a closed room is over: their session is cleared here rather than left
// pointing at a room they cannot rejoin, because the alternative is a session
// that keeps offering "return to your table" for a table that is gone.
//
// ok is false when this function has already written the response.
func (a *App) loadRoomMember(w http.ResponseWriter, r *http.Request, from roomSource) (queries.GetRoomRow, room.Role, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	raw := r.PathValue("id")
	if from == roomFromQuery {
		raw = r.URL.Query().Get("room")
	}

	roomID, err := ulid.Parse(raw)
	if err != nil {
		refuseRoom(w, r, from)
		return queries.GetRoomRow{}, "", false
	}

	row, err := a.Queries.GetRoom(ctx, roomID)
	if errors.Is(err, sql.ErrNoRows) {
		refuseRoom(w, r, from)
		return queries.GetRoomRow{}, "", false
	}
	if err != nil {
		slog.Error("Failed to load room", "error", err)
		if from == roomFromQuery {
			htmx.ServerError(w)
		} else {
			redirectToError(w, r)
		}
		return queries.GetRoomRow{}, "", false
	}

	if row.OwnerID == sess.UserID {
		return row, room.RoleGM, true
	}

	if sess.RoomID == nil || *sess.RoomID != roomID {
		refuseRoom(w, r, from)
		return queries.GetRoomRow{}, "", false
	}

	if row.ClosedAt.Valid {
		a.dismissFromClosedRoom(w, r, &sess, from)
		return queries.GetRoomRow{}, "", false
	}

	return row, room.RolePlayer, true
}

// dismissFromClosedRoom is what a player gets when the GM closed the room under
// them. Their session is cleared rather than left pointing at a room they
// cannot rejoin, and they are sent to the join page, which is where they would
// go next anyway.
//
// IT REDIRECTS ON BOTH SHAPES, where every other refusal answers a fragment
// with a 404. The difference is that this is not a request that should not have
// been made -- the player was in the room a moment ago -- so the answer is
// somewhere to go rather than nothing.
//
// THE TOAST IS READ ON THE FRAGMENT PATH. htmx sees the HX-Trigger header and
// raises it after following the redirect; a full page navigation has no channel
// for a message at all, so that path lands on the join page unannounced. The
// header is set once for both rather than branched, because it costs a line and
// the alternative is two.
func (a *App) dismissFromClosedRoom(w http.ResponseWriter, r *http.Request, sess *session.UserSession, from roomSource) {
	if err := a.Sessions.LeaveRoom(r.Context(), sess); err != nil {
		slog.Error("Failed to clear a closed room from a session", "error", err)
	}

	htmx.Toast(w, "That room has closed.")

	if from == roomFromQuery {
		htmx.Redirect(w, "/rooms/join")
		return
	}

	redirect(w, r, "/rooms/join")
}

// refuseRoom is the one answer for every way of not being allowed into a room:
// a malformed id, a room that is not there, a room that is somebody else's, and
// a room that closed under a player.
//
// THE TWO SHAPES ARE THE POINT OF IT BEING A FUNCTION. A page request is a
// browser navigation, so it gets a 303 to the join page -- somewhere to go
// rather than an error about somewhere they cannot. A fragment gets an empty
// 404, because a fragment is a piece of a page that is already open and the
// noSwap config leaves the caller's target untouched for a 4xx; a page-shaped
// body swapped into a panel would be the wreckage instead.
func refuseRoom(w http.ResponseWriter, r *http.Request, from roomSource) {
	if from == roomFromQuery {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	redirect(w, r, "/rooms/join")
}

// roomMembers is who is at the table as the panel draws them. It is the
// database's answer and not a live one: phase 3 replaces this list with the
// room's own state over the socket, and this query stays as what the first
// render draws before the socket has said anything.
func (a *App) roomMembers(ctx context.Context, roomID ulid.ULID) (pages.RoomMembersData, error) {
	rows, err := a.Queries.ListRoomMembers(ctx, &roomID)
	if err != nil {
		return pages.RoomMembersData{}, err
	}

	members := make([]pages.RoomMember, 0, len(rows))
	for _, row := range rows {
		members = append(members, pages.RoomMember{
			Name:      row.Username,
			AvatarURL: row.ProfileImageURL,
			Character: row.CharacterName.String,
		})
	}

	return pages.RoomMembersData{RoomID: roomID.String(), Members: members}, nil
}
