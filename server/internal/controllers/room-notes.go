package controllers

import (
	"net/http"
	"net/url"
	"strconv"

	"tabletopper/internal/htmx"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func (a *App) HexNoteFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, role, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil || a.Hub == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	layer, q, cellR, ok := hexCell(r.URL.Query())
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	note, _ := a.Hub.Note(ctx, row.ID, layer, q, cellR, role)
	data := noteData(row.ID.String(), layer, q, cellR, role, note)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomNote(data))
}
func noteData(roomID string, layer ulid.ULID, q, r int, role room.Role, note *room.HexNote) pages.RoomNoteData {
	data := pages.RoomNoteData{
		RoomID: roomID,
		Layer:  layer.String(),
		Q:      q,
		R:      r,
		IsGM:   role == room.RoleGM,
	}
	if note != nil {
		data.Title, data.Body, data.Revealed, data.Exists = note.Title, note.Body, note.Revealed, true
	}
	return data
}
func hexCell(query url.Values) (ulid.ULID, int, int, bool) {
	layer, err := ulid.Parse(query.Get("layer"))
	if err != nil {
		return ulid.ULID{}, 0, 0, false
	}
	q, ok := cellAxis(query.Get("q"))
	if !ok {
		return ulid.ULID{}, 0, 0, false
	}
	r, ok := cellAxis(query.Get("r"))
	if !ok {
		return ulid.ULID{}, 0, 0, false
	}
	return layer, q, r, true
}
func cellAxis(raw string) (int, bool) {
	v, err := strconv.Atoi(raw)
	if err != nil || v < -room.CellLimit || v > room.CellLimit {
		return 0, false
	}
	return v, true
}
func (a *App) SetHexNote(w http.ResponseWriter, r *http.Request) {
	layer, q, cellR, ok := hexCell(formValues(r))
	if !ok {
		htmx.NotFound(w, "hex")
		return
	}
	row, role, ok := a.dispatchRoom(w, r, "write a hex note", &room.NoteSet{
		Layer: layer, Q: q, R: cellR,
		Title: r.FormValue("title"),
		Body:  r.FormValue("body"),
	})
	if !ok {
		return
	}
	note, _ := a.Hub.Note(r.Context(), row.ID, layer, q, cellR, role)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomNoteActions(noteData(row.ID.String(), layer, q, cellR, role, note)))
}
func (a *App) RevealHexNote(w http.ResponseWriter, r *http.Request) {
	layer, q, cellR, ok := hexCell(formValues(r))
	if !ok {
		htmx.NotFound(w, "hex")
		return
	}
	a.roomCommand(w, r, "share a hex note", &room.NoteReveal{
		Layer: layer, Q: q, R: cellR, Revealed: r.FormValue("revealed") != "",
	})
}
func (a *App) RemoveHexNote(w http.ResponseWriter, r *http.Request) {
	layer, q, cellR, ok := hexCell(r.URL.Query())
	if !ok {
		htmx.NotFound(w, "hex")
		return
	}
	a.roomCommand(w, r, "rub out a hex note", &room.NoteRemove{Layer: layer, Q: q, R: cellR})
}
func formValues(r *http.Request) url.Values {
	if err := r.ParseForm(); err != nil {
		return url.Values{}
	}
	return r.PostForm
}
