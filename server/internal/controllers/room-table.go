package controllers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"tabletopper/internal/htmx"
	"tabletopper/internal/hub"
	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

func (a *App) RoomLayersFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, view, ok := a.gmTable(ctx, r, r.URL.Query().Get("room"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	names := map[ulid.ULID]string{}
	if rows, err := a.Queries.ListReadyMaps(ctx, sess.UserID); err == nil {
		for _, m := range rows {
			names[m.ID] = m.Name
		}
	} else {
		slog.Error("Failed to list the maps for the layer manager", "error", err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomLayers(layersData(row.ID, view, names)))
}
func (a *App) AddLayer(w http.ResponseWriter, r *http.Request) {
	a.layerCommand(w, r, "add a layer", func(_ ulid.ULID) (room.Command, bool) {
		return &room.TableAddLayer{Name: strings.TrimSpace(r.FormValue("name"))}, true
	})
}
func (a *App) RemoveLayer(w http.ResponseWriter, r *http.Request) {
	a.layerCommand(w, r, "remove a layer", func(layer ulid.ULID) (room.Command, bool) {
		return &room.TableRemoveLayer{Layer: layer}, true
	})
}
func (a *App) RenameLayer(w http.ResponseWriter, r *http.Request) {
	a.layerCommand(w, r, "rename a layer", func(layer ulid.ULID) (room.Command, bool) {
		return &room.TableRenameLayer{Layer: layer, Name: strings.TrimSpace(r.FormValue("name"))}, true
	})
}
func (a *App) MoveLayer(w http.ResponseWriter, r *http.Request) {
	a.layerCommand(w, r, "reorder the layers", func(layer ulid.ULID) (room.Command, bool) {
		index, err := strconv.Atoi(strings.TrimSpace(r.FormValue("index")))
		if err != nil {
			return nil, false
		}
		return &room.TableMoveLayer{Layer: layer, Index: index}, true
	})
}
func (a *App) ActivateLayer(w http.ResponseWriter, r *http.Request) {
	a.layerCommand(w, r, "change the active layer", func(layer ulid.ULID) (room.Command, bool) {
		return &room.TableSetActiveLayer{Layer: layer}, true
	})
}
func (a *App) ClearLayerMap(w http.ResponseWriter, r *http.Request) {
	a.layerCommand(w, r, "clear a layer's map", func(layer ulid.ULID) (room.Command, bool) {
		return &room.TableClearLayerMap{Layer: layer}, true
	})
}
func (a *App) SetLayerMap(w http.ResponseWriter, r *http.Request) {
	ok := a.dispatchLayer(w, r, "change a layer's map", func(layer ulid.ULID) (room.Command, bool) {
		asset, err := ulid.Parse(strings.TrimSpace(r.FormValue("asset")))
		if err != nil {
			return nil, false
		}
		return &room.TableSetLayerMap{Layer: layer, AssetID: asset}, true
	})
	if !ok {
		return
	}
	htmx.CloseModal(w)
	w.WriteHeader(http.StatusNoContent)
}
func (a *App) RoomMapsFragment(w http.ResponseWriter, r *http.Request) {
	data, ok := a.pickerMaps(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomMaps(data))
}
func (a *App) RoomMapListFragment(w http.ResponseWriter, r *http.Request) {
	data, ok := a.pickerMaps(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomMapList(data))
}
func (a *App) pickerMaps(w http.ResponseWriter, r *http.Request) (pages.RoomMapsData, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, _, ok := a.gmTable(ctx, r, r.URL.Query().Get("room"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return pages.RoomMapsData{}, false
	}
	layer, err := ulid.Parse(r.URL.Query().Get("layer"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return pages.RoomMapsData{}, false
	}
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(term)) > pages.AssetNameLimit {
		w.WriteHeader(http.StatusNotFound)
		return pages.RoomMapsData{}, false
	}
	rows, err := a.pickerMapRows(ctx, sess.UserID, term)
	if err != nil {
		htmx.ServerError(w)
		return pages.RoomMapsData{}, false
	}
	return mapsData(row.ID, layer, term, rows), true
}
func (a *App) RoomMapCardFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	roomID, layerID, ok := a.pickerLayer(ctx, r, r.URL.Query().Get("room"), r.URL.Query().Get("layer"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	assetID, err := ulid.Parse(r.URL.Query().Get("asset"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	m, err := a.Queries.GetMap(ctx, queries.GetMapParams{ID: assetID, OwnerID: sess.UserID})
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to load a picker card", "error", err, "assetID", assetID.String())
		}
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomMapCard(pickerChoice(roomID, layerID, m)))
}
func (a *App) UploadRoomMap(w http.ResponseWriter, r *http.Request) {
	roomID, layerID, ok := a.pickerLayer(r.Context(), r, r.PathValue("id"), r.PathValue("layer"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	assetID, filename, ok := a.storeMap(w, r)
	if !ok {
		return
	}
	render(w, r, pages.RoomMapCard(pages.RoomMapChoice{
		RoomID:   roomID,
		LayerID:  layerID,
		ID:       assetID.String(),
		Name:     filename,
		FileName: filename,
		State:    queries.AssetsTileStatePending,
	}))
}
func (a *App) RetryRoomMapTiling(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	roomID, layerID, ok := a.pickerLayer(ctx, r, r.PathValue("id"), r.PathValue("layer"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	assetID, err := ulid.Parse(r.PathValue("asset"))
	if err != nil {
		htmx.NotFound(w, "map")
		return
	}
	m, ok := a.requeueMap(w, ctx, sess.UserID, assetID)
	if !ok {
		return
	}
	render(w, r, pages.RoomMapCard(pickerChoice(roomID, layerID, m)))
}
func (a *App) pickerLayer(ctx context.Context, r *http.Request, roomID string, layerID string) (string, string, bool) {
	row, _, ok := a.gmTable(ctx, r, roomID)
	if !ok {
		return "", "", false
	}
	layer, err := ulid.Parse(layerID)
	if err != nil {
		return "", "", false
	}
	return row.ID.String(), layer.String(), true
}
func pickerChoice(roomID string, layerID string, m queries.Asset) pages.RoomMapChoice {
	choice := pages.RoomMapChoice{
		RoomID:    roomID,
		LayerID:   layerID,
		ID:        m.ID.String(),
		Name:      m.Name,
		FileName:  m.FileName,
		Width:     int(m.Width.Int32),
		Height:    int(m.Height.Int32),
		State:     m.TileState.AssetsTileState,
		AutoRetry: willTileAgain(m.TileState.AssetsTileState, m.TileAttempts),
	}
	if m.TileGen != nil {
		choice.Generation = m.TileGen.String()
	}
	return choice
}
func (a *App) pickerMapRows(ctx context.Context, ownerID ulid.ULID, term string) ([]queries.ListPickerMapsRow, error) {
	if term == "" {
		return a.Queries.ListPickerMaps(ctx, ownerID)
	}
	found, err := a.Queries.SearchPickerMaps(ctx, queries.SearchPickerMapsParams{
		OwnerID: ownerID,
		Term:    journalSearchPattern(term),
	})
	if err != nil {
		return nil, err
	}
	rows := make([]queries.ListPickerMapsRow, 0, len(found))
	for _, row := range found {
		rows = append(rows, queries.ListPickerMapsRow(row))
	}
	return rows, nil
}
func (a *App) RoomGridFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	row, view, ok := a.gmTable(ctx, r, r.URL.Query().Get("room"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomGrid(gridData(row.ID, view.Table, nil)))
}
func (a *App) SetRoomGrid(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	row, role, err := a.roomMember(ctx, sess, r.PathValue("id"))
	if err != nil || a.Hub == nil {
		htmx.NotFound(w, "room")
		return
	}
	grid, options, problems := gridForm(r)
	if len(problems) > 0 {
		renderPanelBlock(w, r, pages.RoomGridPanel, problems)
		return
	}
	who := room.Actor{ID: sess.UserID, Role: role}
	cmd := &room.Batch{Commands: []room.Command{&room.TableSetGrid{Grid: grid}, &options}}
	if err := a.Hub.Dispatch(ctx, row.ID, who, cmd); err != nil {
		var refusal *room.Error
		if errors.As(err, &refusal) && refusal.Code == room.CodeInvalid {
			renderPanelBlock(w, r, pages.RoomGridPanel, []string{refusal.Message})
			return
		}
		a.rejectCommand(w, "change the grid", err)
		return
	}
	renderPanelBlock(w, r, pages.RoomGridPanel, nil)
}
func (a *App) ClearTabletop(w http.ResponseWriter, r *http.Request) {
	a.layerCommand(w, r, "clear the tabletop", func(ulid.ULID) (room.Command, bool) {
		return &room.TableClear{}, true
	})
}
func (a *App) viewedLayerCommands(w http.ResponseWriter, r *http.Request, action string, build func(layer ulid.ULID) []room.Command) {
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
	var layer ulid.ULID
	if raw := r.FormValue("layer"); raw != "" {
		layer, err = ulid.Parse(raw)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	} else {
		view, ok := a.Hub.Table(ctx, row.ID)
		if !ok {
			htmx.NotFound(w, "room")
			return
		}
		layer = view.Table.ActiveLayer
	}
	who := room.Actor{ID: sess.UserID, Role: role}
	if err := a.Hub.Dispatch(ctx, row.ID, who, &room.Batch{Commands: build(layer)}); err != nil {
		a.rejectCommand(w, action, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (a *App) layerCommand(w http.ResponseWriter, r *http.Request, action string, build func(layer ulid.ULID) (room.Command, bool)) {
	if a.dispatchLayer(w, r, action, build) {
		w.WriteHeader(http.StatusNoContent)
	}
}
func (a *App) dispatchLayer(w http.ResponseWriter, r *http.Request, action string, build func(layer ulid.ULID) (room.Command, bool)) bool {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	if a.Hub == nil {
		htmx.NotFound(w, "room")
		return false
	}
	row, role, err := a.roomMember(ctx, sess, r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "room")
		return false
	}
	var layer ulid.ULID
	if raw := r.PathValue("layer"); raw != "" {
		layer, err = ulid.Parse(raw)
		if err != nil {
			htmx.NotFound(w, "layer")
			return false
		}
	}
	cmd, ok := build(layer)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return false
	}
	if err := a.Hub.Dispatch(ctx, row.ID, room.Actor{ID: sess.UserID, Role: role}, cmd); err != nil {
		a.rejectCommand(w, action, err)
		return false
	}
	return true
}
func (a *App) gmTable(ctx context.Context, r *http.Request, id string) (queries.GetRoomRow, *hub.TableView, bool) {
	sess := session.FromContext(ctx)
	row, role, err := a.roomMember(ctx, sess, id)
	if err != nil || role != room.RoleGM || a.Hub == nil {
		return queries.GetRoomRow{}, nil, false
	}
	view, ok := a.Hub.Table(ctx, row.ID)
	if !ok {
		return queries.GetRoomRow{}, nil, false
	}
	return row, view, true
}
func layersData(roomID ulid.ULID, view *hub.TableView, names map[ulid.ULID]string) pages.RoomLayersData {
	var refW, refH int
	for _, l := range view.Table.Layers {
		if l.Map != nil {
			refW, refH = l.Map.Width, l.Map.Height
			break
		}
	}
	last := len(view.Table.Layers) - 1
	out := make([]pages.RoomLayer, 0, len(view.Table.Layers))
	for i, l := range view.Table.Layers {
		row := pages.RoomLayer{
			ID:     l.ID.String(),
			Name:   pages.SafeLayerName(l.Name),
			Index:  i,
			Active: l.ID == view.Table.ActiveLayer,
			Bottom: i == 0,
			Top:    i == last,
			Pawns:  view.Pawns[l.ID],
		}
		if l.Map != nil {
			row.MapID = l.Map.AssetID.String()
			row.MapName = names[l.Map.AssetID]
			row.Width = l.Map.Width
			row.Height = l.Map.Height
			row.Mismatch = refW > 0 && (l.Map.Width != refW || l.Map.Height != refH)
		}
		out = append(out, row)
	}
	return pages.RoomLayersData{
		RoomID: roomID.String(),
		Layers: out,
		Full:   len(out) >= room.LayersMax,
	}
}
func mapsData(roomID ulid.ULID, layer ulid.ULID, term string, rows []queries.ListPickerMapsRow) pages.RoomMapsData {
	roomText, layerText := roomID.String(), layer.String()
	out := make([]pages.RoomMapChoice, 0, len(rows))
	for _, m := range rows {
		choice := pages.RoomMapChoice{
			RoomID:    roomText,
			LayerID:   layerText,
			ID:        m.ID.String(),
			Name:      m.Name,
			FileName:  m.FileName,
			Width:     int(m.Width.Int32),
			Height:    int(m.Height.Int32),
			State:     m.TileState.AssetsTileState,
			AutoRetry: willTileAgain(m.TileState.AssetsTileState, m.TileAttempts),
		}
		if m.TileGen != nil {
			choice.Generation = m.TileGen.String()
		}
		out = append(out, choice)
	}
	return pages.RoomMapsData{
		RoomID:  roomText,
		LayerID: layerText,
		Query:   term,
		Maps:    out,
	}
}
func gridData(roomID ulid.ULID, t room.Table, problems []string) pages.RoomGridData {
	return pages.RoomGridData{
		RoomID:             roomID.String(),
		Lines:              string(t.Grid.Lines),
		CellSize:           t.Grid.CellSize,
		OffsetX:            t.Grid.OffsetX,
		OffsetY:            t.Grid.OffsetY,
		Color:              t.Grid.Color,
		Snap:               string(t.Grid.Snap),
		FeetPerCell:        t.Grid.FeetPerCell,
		Diagonals:          string(t.Grid.Diagonals),
		PawnLabels:         string(t.PawnLabels),
		PlayersCanDraw:     t.PlayersCanDraw,
		FogPrefill:         t.FogPrefill,
		InitiativeGrouping: string(t.InitiativeGrouping),
		Errors:             problems,
	}
}
func gridForm(r *http.Request) (room.Grid, room.TableSetOptions, []string) {
	var problems []string
	number := func(field, caption string) int {
		v, err := strconv.Atoi(strings.TrimSpace(r.FormValue(field)))
		if err != nil {
			problems = append(problems, caption+" has to be a whole number.")
			return 0
		}
		return v
	}
	grid := room.Grid{
		Lines:       room.GridLines(r.FormValue("gridLines")),
		CellSize:    number("cellSize", "Cell size"),
		OffsetX:     number("offsetX", "The offset across"),
		OffsetY:     number("offsetY", "The offset down"),
		Color:       strings.ToUpper(strings.TrimSpace(r.FormValue("color"))),
		Snap:        room.Snap(r.FormValue("snap")),
		FeetPerCell: number("feetPerCell", "Feet per cell"),
		Diagonals:   room.Diagonals(r.FormValue("diagonals")),
	}
	if grid.Color != "" && !strings.HasPrefix(grid.Color, "#") {
		grid.Color = "#" + grid.Color
	}
	options := room.TableSetOptions{
		PawnLabels:         room.PawnLabels(r.FormValue("pawnLabels")),
		PlayersCanDraw:     r.FormValue("playersCanDraw") != "",
		InitiativeGrouping: room.InitiativeGrouping(r.FormValue("initiativeGrouping")),
		FogPrefill:         r.FormValue("fogPrefill") != "",
	}
	return grid, options, problems
}
func (a *App) RoomLayerFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)
	id := r.URL.Query().Get("room")
	row, _, err := a.roomMember(ctx, sess, id)
	if err != nil || a.Hub == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	name := ""
	if view, ok := a.Hub.Table(ctx, row.ID); ok && len(view.Table.Layers) > 1 {
		for _, l := range view.Table.Layers {
			if l.ID == view.Table.ActiveLayer {
				name = pages.SafeLayerName(l.Name)
				break
			}
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomLayerName(pages.RoomLayerNameData{RoomID: row.ID.String(), Name: name, Fetched: true}))
}
