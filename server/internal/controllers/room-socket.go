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

	
	
	
	if row.ClosedAt.Valid {
		http.NotFound(w, r)

		return
	}

	
	
	
	

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






func roomCharacter(sess session.UserSession, roomID ulid.ULID) *ulid.ULID {
	if sess.RoomID == nil || *sess.RoomID != roomID {
		return nil
	}

	return sess.CharacterID
}














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













func clearSocketDeadlines(w http.ResponseWriter) {
	controller := http.NewResponseController(w)

	if err := controller.SetReadDeadline(time.Time{}); err != nil {
		slog.Error("Failed to clear the socket read deadline", "error", err)
	}
	if err := controller.SetWriteDeadline(time.Time{}); err != nil {
		slog.Error("Failed to clear the socket write deadline", "error", err)
	}
}

















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
