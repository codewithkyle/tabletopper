package controllers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"log/slog"

	"tabletopper/internal/htmx"
	"tabletopper/internal/hub"
	"tabletopper/internal/queries"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

// THE GM'S TWO CONFIGURATION WINDOWS AND THE ROUTES BEHIND THEM: the layer
// manager, the map picker it opens, and the grid form.
//
// EVERY ONE OF THESE IS HTTP AND NOT A SOCKET COMMAND, and the reason is the
// same one the kick route gives: htmx is what this application's controls are
// made of, and a control that posted over the socket would need a second way to
// confirm, a second way to report a refusal, and a second way to draw a form
// with its errors. The socket carries the protocol; the DOM carries the app.
//
// NOTHING HERE AUTHORISES ANYTHING. Every command in internal/room already
// refuses a non-GM in its own Authorize, refuses a layer that is not there, and
// refuses the last layer in the room. What these handlers do is establish who
// is asking about which room, turn a form into a command, and turn the
// command's refusal into the alert modal. The rule stays one thing in one
// place, and the socket and these routes cannot drift apart.
//
// THE MUTATIONS ANSWER 204 AND REDRAW NOTHING. Every one of them ends in
// table.updated, which reaches every client including the one that asked; the
// manager and the grid form each carry hx-trigger="room:tabletop from:window" and
// refetch themselves from that. A reply carrying the new markup would save a
// round trip and leave a GM's second tab showing the layer list from before.

// RoomLayersFragment is the layer manager.
func (a *App) RoomLayersFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	sess := session.FromContext(ctx)

	row, view, ok := a.gmTable(ctx, r, r.URL.Query().Get("room"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	// A SECOND QUERY, FOR THE NAMES ALONE. MapRef carries what the renderer
	// needs -- the generation and the geometry -- and deliberately not the
	// asset's name, which is a row the owner can rename at any time and which
	// no client needs to fetch a tile. The manager is the one reader that wants
	// it, so the manager is where it is read.
	//
	// A map on a layer that is NOT in this list is one that has been deleted or
	// is being re-tiled. That is worth saying in the row rather than papering
	// over, because the symptom otherwise is a floor that renders nothing.
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

// AddLayer puts an empty floor above the ones already there.
func (a *App) AddLayer(w http.ResponseWriter, r *http.Request) {
	a.layerCommand(w, r, "add a layer", func(_ ulid.ULID) (room.Command, bool) {
		return &room.TableAddLayer{Name: strings.TrimSpace(r.FormValue("name"))}, true
	})
}

// RemoveLayer deletes a floor and everything standing on it. The confirm modal
// in front of it has already told the GM how many pawns that is.
func (a *App) RemoveLayer(w http.ResponseWriter, r *http.Request) {
	a.layerCommand(w, r, "remove a layer", func(layer ulid.ULID) (room.Command, bool) {
		return &room.TableRemoveLayer{Layer: layer}, true
	})
}

// RenameLayer renames a floor. It is a PATCH because it changes one field of a
// layer that goes on existing, which is the one shape in this file that is not
// a whole-resource write.
func (a *App) RenameLayer(w http.ResponseWriter, r *http.Request) {
	a.layerCommand(w, r, "rename a layer", func(layer ulid.ULID) (room.Command, bool) {
		return &room.TableRenameLayer{Layer: layer, Name: strings.TrimSpace(r.FormValue("name"))}, true
	})
}

// MoveLayer reorders the stack. index is where the layer ends up, counted from
// the bottom, and the core refuses one that is not a position.
func (a *App) MoveLayer(w http.ResponseWriter, r *http.Request) {
	a.layerCommand(w, r, "reorder the layers", func(layer ulid.ULID) (room.Command, bool) {
		index, err := strconv.Atoi(strings.TrimSpace(r.FormValue("index")))
		if err != nil {
			return nil, false
		}

		return &room.TableMoveLayer{Layer: layer, Index: index}, true
	})
}

// ActivateLayer changes what the players are looking at.
func (a *App) ActivateLayer(w http.ResponseWriter, r *http.Request) {
	a.layerCommand(w, r, "change the active layer", func(layer ulid.ULID) (room.Command, bool) {
		return &room.TableSetActiveLayer{Layer: layer}, true
	})
}

// ClearLayerMap empties a layer back to blank. The pawns on it stay, which is
// why this is not the same button as Remove.
func (a *App) ClearLayerMap(w http.ResponseWriter, r *http.Request) {
	a.layerCommand(w, r, "clear a layer's map", func(layer ulid.ULID) (room.Command, bool) {
		return &room.TableClearLayerMap{Layer: layer}, true
	})
}

// SetLayerMap points a layer at one of the GM's tiled maps.
//
// IT IS THE ONE ROUTE HERE THAT CLOSES A MODAL, because the picker in front of
// it is a modal: choosing is a single act with an end, unlike the manager and
// the grid form, which are windows somebody keeps open while they work.
//
// THE MAP IS NOT VALIDATED HERE. hub.resolveMap reads the assets row, refuses a
// map belonging to somebody else and refuses one that has not finished tiling,
// because it is the half of the command that has a database. A 404 for another
// GM's map and a "not ready yet" for an untiled one both come back from there
// through rejectCommand, in the protocol's own words.
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

	// The header goes on a 204, which htmx reads before it decides not to swap
	// anything -- the same path every refusal in this file takes to the alert
	// modal. A picker that stayed open after a choice would look like the click
	// had missed.
	htmx.CloseModal(w)
	w.WriteHeader(http.StatusNoContent)
}

// RoomMapsFragment is the picker: the GM's maps that have finished tiling.
func (a *App) RoomMapsFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	row, _, ok := a.gmTable(ctx, r, r.URL.Query().Get("room"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	layer, err := ulid.Parse(r.URL.Query().Get("layer"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	// THE LIBRARY IS THE ASKER'S OWN, scoped by the session rather than by the
	// room's owner column. They are the same person -- gmTable answered, so
	// this session owns the room -- and scoping by the session is the property
	// every other asset route in this application has.
	rows, err := a.Queries.ListReadyMaps(ctx, sess.UserID)
	if err != nil {
		htmx.ServerError(w)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomMaps(mapsData(row.ID, layer, rows)))
}

// RoomGridFragment is the grid and the two room-wide options, pre-filled from
// the live table.
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

// SetRoomGrid saves the grid and the options together.
//
// TWO COMMANDS, IN ORDER, because they are two singletons in the protocol and
// one form. The grid goes first: it is the one that can be refused, and a form
// that had already written the options before failing would leave the GM with
// half of what they pressed Save for.
//
// THE REFUSAL IS RENDERED INTO THE FORM AND NOT INTO THE ALERT MODAL, unlike
// every other route in this file. This is a form with fields, so "Cell size
// must be between 8 and 512 pixels" belongs above the field that says 4 -- the
// same shape the character panels use, down to the 422 the noSwap list would
// otherwise swallow.
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

	if err := a.Hub.Dispatch(ctx, row.ID, who, &room.TableSetGrid{Grid: grid}); err != nil {
		var refusal *room.Error
		if errors.As(err, &refusal) && refusal.Code == room.CodeInvalid {
			renderPanelBlock(w, r, pages.RoomGridPanel, []string{refusal.Message})

			return
		}

		a.rejectCommand(w, "change the grid", err)

		return
	}

	if err := a.Hub.Dispatch(ctx, row.ID, who, &options); err != nil {
		a.rejectCommand(w, "change the table options", err)

		return
	}

	// AN EMPTY ERROR BLOCK RATHER THAN 204, which is the one place this file
	// answers with markup. The form's errors are swapped into a block that
	// stays on screen until something replaces it, so a save that works has to
	// clear what the save before it left there.
	renderPanelBlock(w, r, pages.RoomGridPanel, nil)
}

// layerCommand is the shape every mutation in this file has: establish the
// asker and the room, read the layer out of the path, build the command, send
// it, and answer 204 or the refusal.
//
// build answers false for a form this server did not write -- an index that is
// not a number, an asset id that is not an id. That is a 404 rather than a
// message, because the only thing that produces one of those is somebody
// posting by hand, and there is nothing to tell them that is not a hint.
func (a *App) layerCommand(w http.ResponseWriter, r *http.Request, action string, build func(layer ulid.ULID) (room.Command, bool)) {
	if a.dispatchLayer(w, r, action, build) {
		w.WriteHeader(http.StatusNoContent)
	}
}

// dispatchLayer is layerCommand without the answer, so the one route that has
// something to say on success -- the picker, which closes its modal -- can say
// it. It writes the whole response on failure and nothing at all on success,
// which is what lets the caller set a header before the status goes out.
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

	// The layer is empty on the add route, which has no layer in its path, and
	// a parse failure there is not a failure -- so it is only insisted on when
	// the pattern actually carries one.
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

// gmTable is the three questions every fragment here asks: is this a room this
// session is in, is this session its GM, and what is on its table.
//
// A PLAYER GETS THE SAME 404 A STRANGER DOES. These fragments are the GM's
// controls; a player who requested one would be told the shape of the room's
// configuration, and there is no reading of that which is not a leak.
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

// layersData turns the live table into the manager's rows.
//
// THE ORDER IS THE STATE'S, WHICH IS BOTTOM TO TOP. A layer stack in a drawing
// application lists the top one first because the top one is what you see; a
// building lists the ground floor first because that is what a floor plan is.
// This is a building.
//
// THE SIZE WARNING MEASURES AGAINST THE FIRST MAPPED LAYER, not against the
// first layer. The grid is room-wide on the assumption that a building's floors
// were exported at one scale, and an empty ground floor is not evidence about
// that either way -- so the reference is the first floor that actually has a
// map, and every other mapped floor is compared with it.
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

// mapsData turns the owner's ready maps into the picker's cards.
func mapsData(roomID ulid.ULID, layer ulid.ULID, rows []queries.ListReadyMapsRow) pages.RoomMapsData {
	out := make([]pages.RoomMapChoice, 0, len(rows))
	for _, m := range rows {
		out = append(out, pages.RoomMapChoice{
			ID:     m.ID.String(),
			Name:   m.Name,
			Width:  int(m.Width.Int32),
			Height: int(m.Height.Int32),
		})
	}

	return pages.RoomMapsData{RoomID: roomID.String(), LayerID: layer.String(), Maps: out}
}

// gridData flattens the table into the grid form.
func gridData(roomID ulid.ULID, t room.Table, problems []string) pages.RoomGridData {
	return pages.RoomGridData{
		RoomID:         roomID.String(),
		Visible:        t.Grid.Visible,
		CellSize:       t.Grid.CellSize,
		OffsetX:        t.Grid.OffsetX,
		OffsetY:        t.Grid.OffsetY,
		Color:          t.Grid.Color,
		Snap:           string(t.Grid.Snap),
		FeetPerCell:    t.Grid.FeetPerCell,
		Diagonals:      string(t.Grid.Diagonals),
		MonsterHP:      string(t.MonsterHP),
		PlayersCanDraw: t.PlayersCanDraw,
		Errors:         problems,
	}
}

// gridForm reads the grid form.
//
// IT VALIDATES SHAPE AND NOTHING ELSE. Whether 4 is a legal cell size is
// internal/room's question and it answers it with a sentence this handler
// prints; what is answered here is whether "4" is a number at all, because a
// field that is not one has no value to send and the core would be asked about
// a zero the reader never typed.
//
// A CHECKBOX THAT IS OFF SENDS NOTHING. That is how HTML forms work and it is
// why both toggles are read by presence rather than by value -- and why this
// function has to see the whole form rather than one field, since "absent"
// means false only when the rest of the form arrived.
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
		Visible:     r.FormValue("showGrid") != "",
		CellSize:    number("cellSize", "Cell size"),
		OffsetX:     number("offsetX", "The offset across"),
		OffsetY:     number("offsetY", "The offset down"),
		Color:       strings.ToUpper(strings.TrimSpace(r.FormValue("color"))),
		Snap:        room.Snap(r.FormValue("snap")),
		FeetPerCell: number("feetPerCell", "Feet per cell"),
		Diagonals:   room.Diagonals(r.FormValue("diagonals")),
	}

	// The hash is optional in the field and required in the state, so it is put
	// back here rather than made the reader's problem. Everything else about
	// the colour -- the length, the digits -- is checkColor's.
	if grid.Color != "" && !strings.HasPrefix(grid.Color, "#") {
		grid.Color = "#" + grid.Color
	}

	options := room.TableSetOptions{
		MonsterHP:      room.HPVisibility(r.FormValue("monsterHp")),
		PlayersCanDraw: r.FormValue("playersCanDraw") != "",
	}

	return grid, options, problems
}

// RoomLayerFragment is the active layer's name, for the menu bar, for anybody
// in the room.
//
// IT IS THE ONE THING IN THIS FILE A PLAYER MAY FETCH, and it is one string:
// which floor the table is on. A player who cannot see the layer list can still
// see which of them they are standing on, which is the difference between a
// room that changed under them and a room that is broken.
//
// AN EMPTY ANSWER IS THE NORMAL ONE. A room with a single layer has no floor to
// name, so the bar carries nothing rather than permanently labelling the only
// thing there is.
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
	render(w, r, pages.RoomLayerName(pages.RoomLayerNameData{RoomID: row.ID.String(), Name: name}))
}
