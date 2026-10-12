package controllers

import (
	"log/slog"
	"net/http"
	"strings"

	"tabletopper/internal/htmx"
	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func (a *App) RoomPaletteFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	part := r.URL.Query().Get("part")
	if part != "" && part != "bag" && part != "shelf" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(term)) > pages.AssetNameLimit {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	row, view, ok := a.gmTable(w, r)
	if !ok {
		return
	}
	var rows []queries.Asset
	if part != "bag" {
		found, err := a.libraryAssets(ctx, sess.UserID, queries.AssetsTypeTerrain, term)
		if err != nil {
			slog.Error("Failed to read the terrain shelf", "error", err)
			htmx.ServerError(w)
			return
		}
		rows = found
	}
	data := paletteData(row.ID.String(), term, view.Table.Palette, rows)
	switch part {
	case "bag":
		renderFragment(w, r, pages.RoomPaletteBag(data))
	case "shelf":
		renderFragment(w, r, pages.RoomPaletteShelf(data))
	default:
		renderFragment(w, r, pages.RoomPalette(data))
	}
}
func paletteData(roomID string, term string, palette []room.TileArt, rows []queries.Asset) pages.RoomPaletteData {
	data := pages.RoomPaletteData{RoomID: roomID, Query: term}
	held := map[ulid.ULID]bool{}
	for _, art := range palette {
		held[art.AssetID] = true
		data.Entries = append(data.Entries, pages.PaletteEntry{
			RoomID: roomID,
			ID:     art.ID.String(),
			Name:   art.Name,
			Image:  art.Image,
		})
	}
	for _, asset := range rows {
		data.Shelf = append(data.Shelf, pages.PaletteChoice{
			RoomID: roomID,
			ID:     asset.ID.String(),
			Name:   asset.Name,
			Image:  "/assets/images/" + asset.ID.String(),
			Held:   held[asset.ID],
		})
	}
	return data
}
func (a *App) AddToPalette(w http.ResponseWriter, r *http.Request) {
	asset, err := ulid.Parse(strings.TrimSpace(r.FormValue("asset")))
	if err != nil {
		htmx.NotFound(w, "terrain picture")
		return
	}
	a.roomCommand(w, r, "add terrain to the palette", &room.PaletteAdd{Asset: asset})
}
func (a *App) RemoveFromPalette(w http.ResponseWriter, r *http.Request) {
	art, ok := pathID(w, r, "art", "terrain picture")
	if !ok {
		return
	}
	a.roomCommand(w, r, "remove terrain from the palette", &room.PaletteRemove{Art: art})
}
func (a *App) roomCommand(w http.ResponseWriter, r *http.Request, action string, cmd room.Command) {
	if _, _, ok := a.dispatchRoom(w, r, action, cmd); ok {
		w.WriteHeader(http.StatusNoContent)
	}
}
func (a *App) dispatchRoom(w http.ResponseWriter, r *http.Request, action string, cmd room.Command) (queries.GetRoomRow, room.Role, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, role, ok := a.liveMember(w, r)
	if !ok {
		return queries.GetRoomRow{}, "", false
	}
	who := room.Actor{ID: sess.UserID, Role: role}
	if err := a.Hub.Dispatch(ctx, row.ID, who, cmd); err != nil {
		a.rejectCommand(w, action, err)
		return queries.GetRoomRow{}, "", false
	}
	return row, role, true
}
func (a *App) ClearLayerTiles(w http.ResponseWriter, r *http.Request) {
	a.viewedLayerCommands(w, r, "clear the tiles", func(layer ulid.ULID) []room.Command {
		return []room.Command{&room.TilesClear{Layer: layer}}
	})
}
