package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func (a *App) RoomScenesFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, _, ok := a.gmTable(ctx, r, r.URL.Query().Get("room"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	rows, err := a.Queries.ListScenes(ctx, sess.UserID)
	if err != nil {
		slog.Error("Failed to list the scenes for the shelf", "error", err)
		htmx.ServerError(w)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomScenes(scenesData(row, sess, rows)))
}
func (a *App) RoomSceneSaveFragment(w http.ResponseWriter, r *http.Request) {
	row, _, ok := a.gmTable(r.Context(), r, r.URL.Query().Get("room"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomSceneSave(pages.RoomSceneSaveData{RoomID: row.ID.String()}))
}
func (a *App) SaveScene(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, role, err := a.roomMember(ctx, sess, r.PathValue("id"))
	if err != nil || a.Hub == nil {
		htmx.NotFound(w, "room")
		return
	}
	if role != room.RoleGM {
		a.rejectCommand(w, "save a scene", forbiddenToPlayers("save a scene"))
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if problem := checkSceneName(name); problem != "" {
		renderPanelBlock(w, r, pages.SceneSavePanel, []string{problem})
		return
	}
	count, err := a.Queries.CountScenes(ctx, sess.UserID)
	if err != nil {
		slog.Error("Failed to count the scenes before saving one", "error", err)
		htmx.ServerError(w)
		return
	}
	if count >= room.ScenesMax {
		renderPanelBlock(w, r, pages.SceneSavePanel, []string{scenesFullMessage()})
		return
	}
	export, ok := a.Hub.Export(ctx, row.ID)
	if !ok {
		htmx.NotFound(w, "room")
		return
	}
	sceneID := ulid.Make()
	err = a.Queries.CreateScene(ctx, queries.CreateSceneParams{
		ID:        sceneID,
		OwnerID:   sess.UserID,
		Name:      name,
		Body:      export.Body,
		PreviewID: export.Preview,
	})
	if err != nil {
		slog.Error("Failed to save a scene", "error", err)
		htmx.ServerError(w)
		return
	}
	a.adoptScene(ctx, row.ID, sess.UserID, &sceneID)
	htmx.CloseModal(w)
	htmx.Toast(w, name+" is saved.")
	htmx.Scenes(w)
	w.WriteHeader(http.StatusNoContent)
}
func (a *App) OpenScene(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, ok := a.sceneRoom(w, r, "open a scene")
	if !ok {
		return
	}
	sceneID, ok := sceneFromPath(w, r)
	if !ok {
		return
	}
	saved, err := a.Queries.GetScene(ctx, queries.GetSceneParams{ID: sceneID, OwnerID: sess.UserID})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "scene")
		return
	}
	if err != nil {
		slog.Error("Failed to read a scene back", "scene", sceneID, "error", err)
		htmx.ServerError(w)
		return
	}
	scene, err := room.Unmarshal(saved.Body)
	if err != nil {
		slog.Error("Failed to decode a scene", "scene", sceneID, "error", err)
		htmx.Error(w, "Scene unreadable", "That scene was saved by a version of the server this one cannot read.", http.StatusUnprocessableEntity)
		return
	}
	was := row.SceneID
	a.autosaveScene(ctx, row.ID, sess.UserID, was)
	a.adoptScene(ctx, row.ID, sess.UserID, &sceneID)
	cmd := &room.SceneLoad{Scene: scene}
	if err := a.Hub.Dispatch(ctx, row.ID, room.Actor{ID: sess.UserID, Role: room.RoleGM}, cmd); err != nil {
		a.adoptScene(ctx, row.ID, sess.UserID, was)
		a.rejectCommand(w, "open a scene", err)
		return
	}
	htmx.Toast(w, openedMessage(saved.Name, cmd.Missing))
	htmx.Scenes(w)
	w.WriteHeader(http.StatusNoContent)
}
func openedMessage(name string, missing []string) string {
	if len(missing) == 0 {
		return name + " is open."
	}
	return name + " is open, without these maps. " + strings.Join(missing, " ")
}
func (a *App) SaveSceneChanges(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, ok := a.sceneRoom(w, r, "save a scene")
	if !ok {
		return
	}
	sceneID, err := ulid.Parse(r.PathValue("scene"))
	if err != nil || row.SceneID == nil || *row.SceneID != sceneID {
		htmx.NotFound(w, "scene")
		return
	}
	saved, err := a.Queries.GetScene(ctx, queries.GetSceneParams{ID: sceneID, OwnerID: sess.UserID})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "scene")
		return
	}
	if err != nil {
		slog.Error("Failed to read a scene back", "scene", sceneID, "error", err)
		htmx.ServerError(w)
		return
	}
	if err := a.putSceneBody(ctx, row.ID, sess.UserID, sceneID); err != nil {
		slog.Error("Failed to write a scene back", "scene", sceneID, "error", err)
		htmx.ServerError(w)
		return
	}
	htmx.Toast(w, saved.Name+" is saved.")
	htmx.Scenes(w)
	w.WriteHeader(http.StatusNoContent)
}
func (a *App) SetSceneAutosave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	sceneID, err := ulid.Parse(r.PathValue("scene"))
	if err != nil {
		htmx.NotFound(w, "scene")
		return
	}
	result, err := a.Queries.SetSceneAutosave(ctx, queries.SetSceneAutosaveParams{
		Autosave: r.FormValue("autosave") != "",
		ID:       sceneID,
		OwnerID:  sess.UserID,
	})
	if !a.sceneWritten(w, result, err, "set a scene's autosave flag", sceneID) {
		return
	}
	htmx.Scenes(w)
	w.WriteHeader(http.StatusNoContent)
}
func (a *App) RenameScene(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	sceneID, ok := sceneFromPath(w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if problem := checkSceneName(name); problem != "" {
		htmx.Error(w, "Name required", problem, http.StatusUnprocessableEntity)
		return
	}
	result, err := a.Queries.RenameScene(ctx, queries.RenameSceneParams{
		Name: name, ID: sceneID, OwnerID: sess.UserID,
	})
	if !a.sceneWritten(w, result, err, "rename a scene", sceneID) {
		return
	}
	htmx.Scenes(w)
	w.WriteHeader(http.StatusNoContent)
}
func (a *App) DuplicateScene(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	sceneID, ok := sceneFromPath(w, r)
	if !ok {
		return
	}
	count, err := a.Queries.CountScenes(ctx, sess.UserID)
	if err != nil {
		slog.Error("Failed to count the scenes before copying one", "error", err)
		htmx.ServerError(w)
		return
	}
	if count >= room.ScenesMax {
		htmx.Error(w, "Shelf full", scenesFullMessage(), http.StatusUnprocessableEntity)
		return
	}
	saved, err := a.Queries.GetScene(ctx, queries.GetSceneParams{ID: sceneID, OwnerID: sess.UserID})
	if errors.Is(err, sql.ErrNoRows) {
		htmx.NotFound(w, "scene")
		return
	}
	if err != nil {
		slog.Error("Failed to read a scene back before copying it", "scene", sceneID, "error", err)
		htmx.ServerError(w)
		return
	}
	result, err := a.Queries.DuplicateScene(ctx, queries.DuplicateSceneParams{
		NewID:   ulid.Make(),
		Name:    copyName(saved.Name),
		ID:      sceneID,
		OwnerID: sess.UserID,
	})
	if !a.sceneWritten(w, result, err, "copy a scene", sceneID) {
		return
	}
	htmx.Scenes(w)
	w.WriteHeader(http.StatusNoContent)
}
func (a *App) DeleteScene(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	sceneID, ok := sceneFromPath(w, r)
	if !ok {
		return
	}
	result, err := a.Queries.DeleteScene(ctx, queries.DeleteSceneParams{ID: sceneID, OwnerID: sess.UserID})
	if !a.sceneWritten(w, result, err, "delete a scene", sceneID) {
		return
	}
	if _, err := a.Queries.ForgetScene(ctx, queries.ForgetSceneParams{SceneID: &sceneID, OwnerID: sess.UserID}); err != nil {
		slog.Error("Failed to forget a deleted scene", "scene", sceneID, "error", err)
	}
	htmx.Scenes(w)
	w.WriteHeader(http.StatusNoContent)
}
func sceneFromPath(w http.ResponseWriter, r *http.Request) (ulid.ULID, bool) {
	sceneID, err := ulid.Parse(r.PathValue("scene"))
	if err != nil {
		htmx.NotFound(w, "scene")
		return ulid.ULID{}, false
	}
	return sceneID, true
}
func (a *App) sceneWritten(w http.ResponseWriter, result sql.Result, err error, action string, sceneID ulid.ULID) bool {
	if err != nil {
		slog.Error("Failed to "+action, "scene", sceneID, "error", err)
		htmx.ServerError(w)
		return false
	}
	rows, err := result.RowsAffected()
	if err != nil || rows == 0 {
		htmx.NotFound(w, "scene")
		return false
	}
	return true
}
func copyName(name string) string {
	copied := name + sceneCopySuffix
	if len([]rune(copied)) > pages.SceneNameLimit {
		return string([]rune(copied)[:pages.SceneNameLimit])
	}
	return copied
}

const sceneCopySuffix = " (copy)"

func (a *App) RoomSceneNameFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, _, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	name := ""
	if row.SceneID != nil {
		if saved, err := a.Queries.GetScene(ctx, queries.GetSceneParams{ID: *row.SceneID, OwnerID: row.OwnerID}); err == nil {
			name = saved.Name
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomSceneName(pages.RoomSceneNameData{RoomID: row.ID.String(), Name: name, Fetched: true}))
}
func (a *App) sceneRoom(w http.ResponseWriter, r *http.Request, action string) (queries.GetRoomRow, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, role, err := a.roomMember(ctx, sess, r.PathValue("id"))
	if err != nil || a.Hub == nil {
		htmx.NotFound(w, "room")
		return queries.GetRoomRow{}, false
	}
	if role != room.RoleGM {
		a.rejectCommand(w, action, forbiddenToPlayers(action))
		return queries.GetRoomRow{}, false
	}
	return row, true
}
func (a *App) autosaveScene(ctx context.Context, roomID, ownerID ulid.ULID, sceneID *ulid.ULID) {
	if sceneID == nil {
		return
	}
	saved, err := a.Queries.GetScene(ctx, queries.GetSceneParams{ID: *sceneID, OwnerID: ownerID})
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err != nil {
		slog.Error("Failed to read the open scene back", "scene", sceneID, "error", err)
		return
	}
	if !saved.Autosave {
		return
	}
	if err := a.putSceneBody(ctx, roomID, ownerID, *sceneID); err != nil {
		slog.Error("Failed to autosave a scene", "scene", sceneID, "error", err)
	}
}
func (a *App) putSceneBody(ctx context.Context, roomID, ownerID, sceneID ulid.ULID) error {
	if a.Hub == nil {
		return errRoomNotFound
	}
	export, ok := a.Hub.Export(ctx, roomID)
	if !ok {
		return errRoomNotFound
	}
	_, err := a.Queries.UpdateSceneBody(ctx, queries.UpdateSceneBodyParams{
		Body:      export.Body,
		PreviewID: export.Preview,
		ID:        sceneID,
		OwnerID:   ownerID,
	})
	return err
}
func (a *App) closeScene(ctx context.Context, roomID, ownerID ulid.ULID) {
	row, err := a.Queries.GetRoom(ctx, roomID)
	if err != nil || row.OwnerID != ownerID {
		return
	}
	a.autosaveScene(ctx, roomID, ownerID, row.SceneID)
}
func (a *App) adoptScene(ctx context.Context, roomID, ownerID ulid.ULID, sceneID *ulid.ULID) {
	if _, err := a.Queries.SetRoomScene(ctx, queries.SetRoomSceneParams{
		SceneID: sceneID,
		ID:      roomID,
		OwnerID: ownerID,
	}); err != nil {
		slog.Error("Failed to mark the room's open scene", "room", roomID, "error", err)
	}
}
func checkSceneName(name string) string {
	switch {
	case name == "":
		return "Name is required."
	case len([]rune(name)) > pages.SceneNameLimit:
		return fmt.Sprintf("Name must be %d characters or fewer.", pages.SceneNameLimit)
	}
	return ""
}
func scenesFullMessage() string {
	return fmt.Sprintf("You already have %d scenes saved. Delete one before saving another.", room.ScenesMax)
}
func forbiddenToPlayers(what string) error {
	return &room.Error{
		Code:    room.CodeForbidden,
		Heading: "Only the GM",
		Message: "Only the GM can " + what + ".",
	}
}
func sceneTimestamp(sess session.UserSession, at time.Time) pages.Timestamp {
	iso, text := sess.Prefs.Format(at)
	return pages.Timestamp{ISO: iso, Text: text}
}
func scenesData(row queries.GetRoomRow, sess session.UserSession, rows []queries.ListScenesRow) pages.RoomScenesData {
	open := row.SceneID
	cards := make([]pages.SceneCard, 0, len(rows))
	for _, s := range rows {
		card := pages.SceneCard{
			RoomID:   row.ID.String(),
			ID:       s.ID.String(),
			Name:     s.Name,
			Updated:  sceneTimestamp(sess, s.UpdatedAt),
			Open:     open != nil && *open == s.ID,
			Autosave: s.Autosave,
		}
		if !s.PreviewID.IsZero() {
			card.PreviewID = s.PreviewID.String()
		}
		cards = append(cards, card)
	}
	return pages.RoomScenesData{RoomID: row.ID.String(), Scenes: cards}
}
