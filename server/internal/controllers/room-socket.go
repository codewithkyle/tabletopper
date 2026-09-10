package controllers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"tabletopper/internal/htmx"
	"tabletopper/internal/hub"
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

	characterID := roomCharacter(sess, row.ID)

	a.Hub.Serve(w, r, row.ID, room.Player{
		ID:            sess.UserID,
		Name:          sess.Username,
		Avatar:        sess.ProfileImageURL,
		CharacterID:   characterID,
		CharacterName: a.characterName(ctx, sess.UserID, characterID),
		Role:          role,
	}, a.stillMember(r, row.ID, role))
}

// stillMember is the check the upgrade just ran, packaged so the hub can run
// it again for as long as the socket lives.
//
// IT RE-READS THE SESSION FROM THE COOKIE rather than reusing the one on the
// context, because the cookie is the one thing about the request that stays
// true: the session row behind it is what a logout ends, what a leave in
// another tab clears the room off, and what a kick clears too. A request whose
// cookie no longer names a live session is a socket that should not be open,
// whoever it was opened by.
//
// THE ROLE MUST NOT HAVE CHANGED EITHER. A room has one owner, so it cannot in
// practice -- but the check is one comparison and the failure it would let
// through is a player socket carrying a GM's authority, which is the one
// failure this whole route exists to prevent.
//
// A CLOSED ROOM ENDS THE GM'S SOCKET AS WELL. roomMember still answers the GM
// for a closed room, because the page renders for them with Reopen in the
// menu; the upgrade refuses it separately above, and so does this.
func (a *App) stillMember(r *http.Request, roomID ulid.ULID, role room.Role) hub.Membership {
	return func(ctx context.Context) bool {
		sess, err := a.Sessions.FromRequest(r.WithContext(ctx))
		if err != nil {
			return false
		}

		row, now, err := a.roomMember(ctx, sess, roomID.String())
		if err != nil || now != role || row.ClosedAt.Valid {
			return false
		}

		return true
	}
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

// characterName is the one read this handler does that the room could not do
// for itself, and it happens here because here is the only place it can: the
// room is a goroutine holding its own state, and the name has to be in hand
// before the player row is handed to it.
//
// IT IS ONE STATEMENT PER CONNECTION and not per frame -- a player with three
// tabs open pays for it three times, on the three occasions a socket opens.
//
// AN EMPTY NAME IS A LEGITIMATE ANSWER and never an error the caller sees. The
// GM brings no character, so there is nothing to look up; a player whose
// character was deleted while they were away looks up nothing. Both are drawn
// as the account name alone, and a database that would not answer is logged and
// falls into the same shape rather than refusing the connection over a label.
func (a *App) characterName(ctx context.Context, userID ulid.ULID, characterID *ulid.ULID) string {
	if characterID == nil {
		return ""
	}

	name, err := a.Queries.GetCharacterName(ctx, queries.GetCharacterNameParams{
		ID:      *characterID,
		OwnerID: userID,
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Error("Failed to read a character name for the socket", "error", err)
	}

	return name
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

	row, role, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
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
		CanKick: role == room.RoleGM,
	}))
}

// KickPlayer removes somebody from the table, and it is the GM's only
// moderation tool.
//
// EVERY REFUSAL IS ALREADY WRITTEN AND NONE OF IT IS HERE. PlayerKick's own
// Authorize refuses a non-GM and refuses the GM kicking themselves; its Apply
// refuses somebody who has already gone and refuses the GM as a target. This
// handler establishes who is asking and about which room, and hands the rest to
// the command -- so the rule is one thing in one place, and the socket and this
// route cannot disagree about it.
//
// THE ROLE CHECK HERE IS NOT THE AUTHORIZATION, it is the actor. roomMember
// derives the role from the rooms row, and that role is what Authorize is then
// run against; a player who posts this gets CodeForbidden from the command
// rather than a 404 from here, which is right -- they are a member of the room,
// they simply may not do this.
//
// WHAT ACTUALLY HAPPENS TO THE PERSON is in internal/hub: the room emits
// player.kicked to them alone, the hub closes their sockets with the reason
// "kicked" so the client stops reconnecting, and it clears the room off their
// session rows so the homepage stops offering to take them back. The room bundle
// parks an alert and sends them home; see server/js/room/exit.ts.
func (a *App) KickPlayer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	if a.Hub == nil {
		htmx.NotFound(w, "room")

		return
	}

	row, role, err := a.roomMember(ctx, sess, r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "room")

		return
	}

	playerID, err := ulid.Parse(r.PathValue("player"))
	if err != nil {
		htmx.NotFound(w, "player")

		return
	}

	// THE MEMBERSHIP IS CLEARED FIRST AND SYNCHRONOUSLY, before the room is
	// told. The hub clears it too, on a goroutine of its own -- that is the
	// belt for a kick sent over the socket -- but a tab of the kicked person's
	// in reconnect backoff can arrive at the upgrade in the window between the
	// event and that write, pass the membership check against rows the clear
	// had not yet reached, and be seated again by the join. Writing the rows
	// here, with the request in hand, closes that window from this side; the
	// room's own kick grace closes it from the other.
	//
	// ONLY FOR THE GM. A player posting this is refused by Authorize a moment
	// later, and clearing anybody's rows on their say-so would be the kick
	// happening without the permission check.
	if role == room.RoleGM && playerID != sess.UserID {
		if _, err := a.Queries.ClearUserRoomSessions(ctx, queries.ClearUserRoomSessionsParams{
			RoomID: &row.ID,
			UserID: playerID,
		}); err != nil {
			slog.Error("Failed to clear a kicked player's membership", "error", err)
			htmx.ServerError(w)

			return
		}
	}

	who := room.Actor{ID: sess.UserID, Role: role}
	if err := a.Hub.Dispatch(ctx, row.ID, who, &room.PlayerKick{ID: playerID}); err != nil {
		a.rejectCommand(w, "remove a player", err)

		return
	}

	members, live := a.roomMembers(ctx, row)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomMembers(pages.RoomMembersData{
		RoomID:  row.ID.String(),
		Members: members,
		Live:    live,
		CanKick: role == room.RoleGM,
	}))
}

// rejectCommand turns a refusal from the protocol into the alert modal.
//
// THE PROTOCOL ALREADY WROTE THE SENTENCE. A room.Error carries a heading and a
// message chosen for the person reading it -- "The GM cannot be removed from
// their own room" -- so this hands both to the alert rather than inventing a
// second wording for a rule that is stated once.
//
// Anything that is not a room.Error is the hub failing rather than the command
// being refused: a room that would not load, a context that expired, an actor
// that has gone. Those are 500s with the generic message, and they are logged,
// because there is nothing useful to tell the GM about them.
func (a *App) rejectCommand(w http.ResponseWriter, action string, err error) {
	var refusal *room.Error
	if !errors.As(err, &refusal) {
		slog.Error("Failed to dispatch a room command", "action", action, "error", err)
		htmx.ServerError(w)

		return
	}

	status := http.StatusUnprocessableEntity
	if refusal.Code == room.CodeForbidden {
		status = http.StatusForbidden
	}
	if refusal.Code == room.CodeNotFound {
		status = http.StatusNotFound
	}

	htmx.Error(w, refusal.Heading, refusal.Message, status)
}

// roomMembers asks the hub first and the database second.
func (a *App) roomMembers(ctx context.Context, row queries.GetRoomRow) ([]pages.RoomMember, bool) {
	if a.Hub != nil {
		if players, ok := a.Hub.Players(ctx, row.ID); ok {
			out := make([]pages.RoomMember, 0, len(players))
			for _, p := range players {
				isGM := p.Role == room.RoleGM
				out = append(out, pages.RoomMember{
					ID:        p.ID.String(),
					Name:      pages.MemberName(isGM, p.CharacterName, p.Name),
					Username:  p.Name,
					Avatar:    p.Avatar,
					IsGM:      isGM,
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
		isGM := m.UserID == row.OwnerID
		out = append(out, pages.RoomMember{
			ID:       m.UserID.String(),
			Name:     pages.MemberName(isGM, m.CharacterName.String, m.Username),
			Username: m.Username,
			Avatar:   session.AvatarURL(m.AvatarAssetID, m.ProfileImageURL),
			IsGM:     isGM,
		})
	}

	return pages.SortRoomMembers(out), false
}
