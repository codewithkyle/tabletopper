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
	row, view, ok := a.gmTable(ctx, r, r.URL.Query().Get("room"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	switch part {
	case "bag":
		render(w, r, pages.RoomPaletteBag(data))
	case "shelf":
		render(w, r, pages.RoomPaletteShelf(data))
	default:
		render(w, r, pages.RoomPalette(data))
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
	a.paletteCommand(w, r, "add terrain to the palette", &room.PaletteAdd{Asset: asset})
}
func (a *App) RemoveFromPalette(w http.ResponseWriter, r *http.Request) {
	art, err := ulid.Parse(r.PathValue("art"))
	if err != nil {
		htmx.NotFound(w, "terrain picture")
		return
	}
	a.paletteCommand(w, r, "remove terrain from the palette", &room.PaletteRemove{Art: art})
}
func (a *App) paletteCommand(w http.ResponseWriter, r *http.Request, action string, cmd room.Command) {
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
	who := room.Actor{ID: sess.UserID, Role: role}
	if err := a.Hub.Dispatch(ctx, row.ID, who, cmd); err != nil {
		a.rejectCommand(w, action, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *App) ClearLayerTiles(w http.ResponseWriter, r *http.Request) {
	a.viewedLayerCommands(w, r, "clear the tiles", func(layer ulid.ULID) []room.Command {
		return []room.Command{&room.TilesClear{Layer: layer}}
	})
}
