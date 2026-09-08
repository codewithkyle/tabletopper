package controllers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

// RoomSocket is the room's live connection, and the only route in the app that
// does not answer with a document.
//
// IT HAS A PREFIX OF ITS OWN, /socket/, and not /fragment/ and not the room's
// path. /fragment/ promises partial HTML and this returns no HTML at all; the
// room's own path is where it belongs and is where ServeMux will not let it go,
// because "/rooms/{id}/socket" and "/rooms/join/{code}" both match
// "/rooms/join/socket" with neither more specific. See routes.go.
//
// AUTHENTICATION IS THE SESSION COOKIE AND NOTHING ELSE. The socket is
// same-origin, so the cookie rides the upgrade like it rides every other
// request -- there is no token to mint, hand to the client and expire. The
// library's own Origin check is what keeps another site from opening one; see
// the comment on attach in internal/hub.
//
// A NON-MEMBER GETS 404 AND NOT 403, like every other room route: telling
// somebody "that room exists but is not yours" is a way to enumerate rooms.
func (a *App) RoomSocket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	if a.Hub == nil {
		http.NotFound(w, r)

		return
	}

	row, role, err := a.roomMember(ctx, sess, r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)

		return
	}

	// A closed room has nothing to run. The player half of this was already
	// answered by roomMember, which clears the room off their session; this is
	// the GM's, whose page still renders so they can reopen it.
	if row.ClosedAt.Valid {
		http.NotFound(w, r)

		return
	}

	// THE LOCK IS NOT CHECKED HERE, deliberately. Locking a room means nobody
	// else gets in, and everybody who is in it stays -- so it is the join that
	// asks, in JoinRoomForm, and a member reconnecting after a dropped train
	// tunnel is not joining.

	clearSocketDeadlines(w)

	a.Hub.Serve(w, r, row.ID, room.Player{
		ID:          sess.UserID,
		Name:        sess.Username,
		Avatar:      sess.ProfileImageURL,
		CharacterID: roomCharacter(sess, row.ID),
		Role:        role,
	})
}

// roomCharacter is the character this person joined THIS room with, which is
// not the same question as what their session's character column says. The
// column follows them from table to table; a GM who played somewhere else last
// week would otherwise arrive holding a character that belongs to another
// room's party.
func roomCharacter(sess session.UserSession, roomID ulid.ULID) *ulid.ULID {
	if sess.RoomID == nil || *sess.RoomID != roomID {
		return nil
	}

	return sess.CharacterID
}

// clearSocketDeadlines takes the server's request timeouts off this connection.
//
// A deadline set through the ResponseController overrides the one the server
// established when the request began, which is what makes this work without
// relaxing ReadTimeout and WriteTimeout for every route. The zero time means no
// deadline, which is what a connection that is expected to live for a whole
// session needs -- the five-second write deadline the socket actually enforces
// is per frame and is set by the write pump.
//
// A failure means something between here and net/http wrapped the
// ResponseWriter without an Unwrap method, and the upgrade below is about to be
// cut off after ten seconds with nothing else to explain it.
func clearSocketDeadlines(w http.ResponseWriter) {
	controller := http.NewResponseController(w)

	if err := controller.SetReadDeadline(time.Time{}); err != nil {
		slog.Error("Failed to clear the socket read deadline", "error", err)
	}
	if err := controller.SetWriteDeadline(time.Time{}); err != nil {
		slog.Error("Failed to clear the socket write deadline", "error", err)
	}
}

// RoomMembersFragment is who is at the table, and it is the first of the room's
// live panels: a socket event fires a DOM event, the panel's hx-trigger hears
// it, and this runs. No JSON is rendered in the browser and no markup is sent
// over the socket.
//
// IT IS BEHIND auth.Fragment LIKE EVERY OTHER FRAGMENT, with the membership
// check here rather than in a wrapper of its own. The wrapper's job is "who is
// asking"; which room they are asking about is a query parameter this handler
// parses, so a wrapper would have to parse it too and the check would live in
// two places.
//
// THE FALLBACK IS THE SESSION ROWS AND IT IS ALMOST NEVER REACHED. A room is
// live from the moment somebody's socket opens, so this answers the window
// between the page rendering and its first frame -- and it answers with the
// membership rather than with who is connected, because there is nobody
// connected to a room that is not running.
func (a *App) RoomMembersFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	row, _, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	members, live := a.roomMembers(ctx, row)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomMembers(pages.RoomMembersData{
		RoomID:  row.ID.String(),
		Members: members,
		Live:    live,
	}))
}

// roomMembers asks the hub first and the database second.
func (a *App) roomMembers(ctx context.Context, row queries.GetRoomRow) ([]pages.RoomMember, bool) {
	if a.Hub != nil {
		if players, ok := a.Hub.Players(ctx, row.ID); ok {
			out := make([]pages.RoomMember, 0, len(players))
			for _, p := range players {
				out = append(out, pages.RoomMember{
					Name:      p.Name,
					Avatar:    p.Avatar,
					IsGM:      p.Role == room.RoleGM,
					Connected: p.Connected,
				})
			}

			return pages.SortRoomMembers(out), true
		}
	}

	rows, err := a.Queries.ListRoomMembers(ctx, &row.ID)
	if err != nil {
		slog.Error("Failed to list room members", "error", err)

		return nil, false
	}

	out := make([]pages.RoomMember, 0, len(rows))
	for _, m := range rows {
		out = append(out, pages.RoomMember{
			Name:   m.Username,
			Avatar: m.ProfileImageURL,
			IsGM:   m.UserID == row.OwnerID,
		})
	}

	return pages.SortRoomMembers(out), false
}
