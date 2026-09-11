package controllers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func (a *App) JoinRoomPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	code := room.NormalizeCode(r.PathValue("code"))
	if !room.ValidCode(code) {
		code = ""
	}
	characters, err := a.Queries.GetCharacters(ctx, sess.UserID)
	if err != nil {
		slog.Error("Failed to load characters for the join page", "error", err)
		redirectToError(w, r)
		return
	}
	options := make([]pages.JoinCharacterOption, 0, len(characters))
	for _, character := range characters {
		options = append(options, pages.JoinCharacterOption{
			ID:   character.ID.String(),
			Name: character.Name,
		})
	}
	render(w, r, pages.JoinRoom(pages.JoinRoomPageData{Code: code, Characters: options}))
}
func (a *App) JoinRoomForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	if err := r.ParseForm(); err != nil {
		rejectJoin(w, r, "The submitted form data could not be read.", http.StatusUnprocessableEntity)
		return
	}
	code := room.NormalizeCode(r.PostFormValue("code"))
	if !room.ValidCode(code) {
		rejectJoin(w, r, "A room code is four letters or numbers.", http.StatusUnprocessableEntity)
		return
	}
	if !a.RoomJoinAttempts.Allow(sess.UserID.String(), time.Now()) {
		rejectJoin(w, r, "Too many attempts. Wait a minute and try again.", http.StatusTooManyRequests)
		return
	}
	characterID, ok := a.joiningCharacter(w, r)
	if !ok {
		return
	}
	found, err := a.Queries.GetOpenRoomByCode(ctx, sql.NullString{String: code, Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		rejectJoin(w, r, "No open room has that code.", http.StatusUnprocessableEntity)
		return
	}
	if err != nil {
		slog.Error("Failed to look up a room by code", "error", err)
		htmx.ServerError(w)
		return
	}
	if found.IsLocked {
		rejectJoin(w, r, "That room is locked. Ask the GM to unlock it.", http.StatusUnprocessableEntity)
		return
	}
	if err := a.Sessions.JoinRoom(ctx, &sess, found.ID, characterID); err != nil {
		slog.Error("Failed to join room", "error", err)
		htmx.ServerError(w)
		return
	}
	htmx.Toast(w, "You joined "+found.Name+".")
	htmx.Redirect(w, "/rooms/"+found.ID.String())
}
func (a *App) joiningCharacter(w http.ResponseWriter, r *http.Request) (*ulid.ULID, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	raw := r.PostFormValue("character")
	if raw == pages.NoCharacterValue {
		rejectJoin(w, r, "Choose the character you are playing.", http.StatusUnprocessableEntity)
		return nil, false
	}
	characterID, err := ulid.Parse(raw)
	if err != nil {
		rejectJoin(w, r, "That character is not yours.", http.StatusUnprocessableEntity)
		return nil, false
	}
	_, err = a.Queries.GetCharacterName(ctx, queries.GetCharacterNameParams{ID: characterID, OwnerID: sess.UserID})
	if errors.Is(err, sql.ErrNoRows) {
		rejectJoin(w, r, "That character is not yours.", http.StatusUnprocessableEntity)
		return nil, false
	}
	if err != nil {
		slog.Error("Failed to verify a character for a join", "error", err)
		htmx.ServerError(w)
		return nil, false
	}
	return &characterID, true
}
func rejectJoin(w http.ResponseWriter, r *http.Request, message string, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	render(w, r, pages.PanelFormErrors(pages.JoinRoomPanel, []string{message}))
}
