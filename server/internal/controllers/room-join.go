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

// JoinRoomPage is the code field and the character picker. It serves both
// patterns: the bare /rooms/join, and /rooms/join/{code} for a link somebody
// pasted into chat.
//
// THE CODE IN THE PATH PREFILLS AND NEVER JOINS. A GET that seated somebody at
// a table would be a state change behind a link -- the same rule that put
// /logout on POST, and it matters more here, because a room code travels in
// exactly the kind of message a link preview crawler follows.
//
// A code in the path that is not shaped like one is ignored rather than
// refused. The page is real either way, the field is empty, and there is
// nothing useful to say to somebody whose friend mistyped a link.
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

// JoinRoomForm seats this session at the room the code names.
//
// THE ORDER OF THE CHECKS IS THE DESIGN. The code's shape is checked first,
// because a value that cannot name a room does not need a query run to find
// that out -- the same refusal share.ValidToken makes in front of every share
// route. The character is verified next, so a caller cannot use a bad character
// to probe codes without paying the rate limit. Only then is a try counted, and
// only then is the code looked up.
//
// THE COUNTER IS KEYED BY THE USER AND NOT BY THE CODE, and the refusal is
// counted like any other try -- see App.RoomJoinAttempts and share.Attempts for
// both halves of why.
//
// A LOCKED ROOM IS TOLD IT IS LOCKED, which the share design deliberately
// avoids for tokens. A room code is not a bearer credential: knowing one admits
// you to a table where the GM can see you and remove you, so leaking that a
// code is in use costs little, and the friend who typed the right code and was
// still turned away is who the message is for.
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

	characterID, ok := a.joiningCharacter(w, r)
	if !ok {
		return
	}

	if !a.RoomJoinAttempts.Allow(sess.UserID.String(), time.Now()) {
		rejectJoin(w, r, "Too many attempts. Wait a minute and try again.", http.StatusTooManyRequests)
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

// joiningCharacter reads the picker. An empty value is "No character", which is
// a legitimate answer -- a player may join before they have made one -- and any
// other value has to be a character on this account's own roster.
//
// IT IS CHECKED AGAINST THE ROSTER RATHER THAN TRUSTED, because the id arrives
// off a form and the session that will carry it is what a pawn is spawned from
// later. GetCharacter is owner-scoped, so somebody else's character matches
// nothing and is refused with the same message a made-up id gets.
//
// ok is false when this function has already written the response.
func (a *App) joiningCharacter(w http.ResponseWriter, r *http.Request) (*ulid.ULID, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	raw := r.PostFormValue("character")
	if raw == pages.NoCharacterValue {
		return nil, true
	}

	characterID, err := ulid.Parse(raw)
	if err != nil {
		rejectJoin(w, r, "That character is not yours.", http.StatusUnprocessableEntity)
		return nil, false
	}

	_, err = a.Queries.GetCharacter(ctx, queries.GetCharacterParams{ID: characterID, OwnerID: sess.UserID})
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

// rejectJoin answers with the form's error block, leaving the form itself
// alone -- so the code the player typed and the character they picked are still
// on screen under the message.
//
// TWO STATUSES REACH HERE AND THE FORM HAS A ROUTE FOR BOTH. 422 is the one
// every other form in the app uses; the rate limit answers 429, because that is
// what it is, and the form carries an hx-status:429 beside its 422 so the
// message lands rather than being swallowed by the noSwap list.
func rejectJoin(w http.ResponseWriter, r *http.Request, message string, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	render(w, r, pages.PanelFormErrors(pages.JoinRoomPanel, []string{message}))
}
