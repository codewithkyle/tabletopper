package controllers

import (
	"context"
	"database/sql"
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

// WHAT IS ON THE TABLE, AND THE ROUTES THAT CHANGE IT.
//
// EVERY ONE OF THESE IS HTTP AND NOT A SOCKET COMMAND, which is room-table.go's
// rule for room-table.go's reason: htmx is what this application's controls are
// made of, and a control that posted over the socket would need a second way to
// confirm, a second way to report a refusal, and a second way to draw a form
// with its errors. The socket carries what originates on the CANVAS -- a drag,
// a placement click -- and the DOM carries everything else.
//
// NOTHING HERE AUTHORISES ANYTHING, with one exception. Every command in
// internal/room refuses the wrong actor in its own Authorize, and these
// handlers turn a form into a command and a refusal into the alert modal. The
// exception is the three fragments, which answer a question no command asks --
// "may you LOOK at this" -- and that is where the projection comes in.
//
// THE PROJECTION IS THE ONE THING IN THIS FILE THAT MUST NOT BE GOT WRONG.
// hub.Pawn takes a role and answers with the copy that role may see, or
// nothing. A handler that reached past it would be a door around the whole
// two-audience design: a player guesses a ULID, GETs the panel, and reads a
// hidden monster's hit points that the socket was careful never to send. There
// is deliberately no accessor that hands back the stored pawn.

// spawnKinds is the whole set the spawn dialog accepts, matched before anything
// reaches a statement.
var spawnKinds = map[string]bool{
	pages.RoomSpawnMonsters: true,
	pages.RoomSpawnTokens:   true,
}

// RoomSpawnFragment is the Spawn dialog, in the content modal.
func (a *App) RoomSpawnFragment(w http.ResponseWriter, r *http.Request) {
	data, ok := a.spawnData(w, r)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomSpawn(data))
}

// RoomSpawnListFragment is the results grid alone, which is what a search
// replaces. It exists for the reason the map picker's list route does: a search
// that swapped the whole dialog would swap the box being typed into.
func (a *App) RoomSpawnListFragment(w http.ResponseWriter, r *http.Request) {
	data, ok := a.spawnData(w, r)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomSpawnList(data))
}

// spawnData is the query behind both. A false return has already answered.
func (a *App) spawnData(w http.ResponseWriter, r *http.Request) (pages.RoomSpawnData, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	row, _, ok := a.gmTable(ctx, r, r.URL.Query().Get("room"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)

		return pages.RoomSpawnData{}, false
	}

	kind := r.URL.Query().Get("kind")
	if !spawnKinds[kind] {
		w.WriteHeader(http.StatusNotFound)

		return pages.RoomSpawnData{}, false
	}

	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(term)) > pages.AssetNameLimit {
		w.WriteHeader(http.StatusNotFound)

		return pages.RoomSpawnData{}, false
	}

	data := pages.RoomSpawnData{RoomID: row.ID.String(), Kind: kind, Query: term}

	if kind == pages.RoomSpawnMonsters {
		monsters, err := a.spawnMonsters(ctx, sess.UserID, term)
		if err != nil {
			slog.Error("Failed to list monsters for the spawn dialog", "error", err)
			htmx.ServerError(w)

			return pages.RoomSpawnData{}, false
		}
		data.Monsters = monsters

		return data, true
	}

	tokens, err := a.spawnTokens(ctx, sess.UserID, term)
	if err != nil {
		slog.Error("Failed to list tokens for the spawn dialog", "error", err)
		htmx.ServerError(w)

		return pages.RoomSpawnData{}, false
	}
	data.Tokens = tokens

	return data, true
}

// spawnMonsters is the manual, whole or searched, as the pick cards read it.
// It goes through monsterSummary so a card in this dialog and a card on the
// manual page are the same five values assembled the same way.
func (a *App) spawnMonsters(ctx context.Context, ownerID ulid.ULID, term string) ([]pages.MonsterSummary, error) {
	rows, err := a.monsterRows(ctx, ownerID, term)
	if err != nil {
		return nil, err
	}

	out := make([]pages.MonsterSummary, 0, len(rows))
	for _, m := range rows {
		out = append(out, monsterSummary(m))
	}

	return out, nil
}

func (a *App) monsterRows(ctx context.Context, ownerID ulid.ULID, term string) ([]queries.Monster, error) {
	if term == "" {
		return a.Queries.ListMonsters(ctx, ownerID)
	}

	return a.Queries.SearchMonsters(ctx, queries.SearchMonstersParams{
		OwnerID: ownerID,
		Term:    journalSearchPattern(term),
	})
}

// spawnTokens is the token library, whole or searched.
func (a *App) spawnTokens(ctx context.Context, ownerID ulid.ULID, term string) ([]pages.RoomSpawnToken, error) {
	var rows []queries.Asset
	var err error

	if term == "" {
		rows, err = a.Queries.GetLibraryAssets(ctx, queries.GetLibraryAssetsParams{
			OwnerID: ownerID,
			Type:    queries.AssetsTypeToken,
		})
	} else {
		rows, err = a.Queries.SearchLibraryAssets(ctx, queries.SearchLibraryAssetsParams{
			OwnerID: ownerID,
			Type:    queries.AssetsTypeToken,
			Term:    journalSearchPattern(term),
		})
	}
	if err != nil {
		return nil, err
	}

	out := make([]pages.RoomSpawnToken, 0, len(rows))
	for _, t := range rows {
		out = append(out, pages.RoomSpawnToken{
			ID:     t.ID.String(),
			Name:   t.Name,
			Image:  "/assets/images/" + t.ID.String(),
			Width:  int(t.Width.Int32),
			Height: int(t.Height.Int32),
		})
	}

	return out, nil
}

// SpawnParty places a pawn for everybody connected who joined with a character.
// It is the Tabletop menu's Spawn pawns item, and it is the only way a player
// character reaches the table.
//
// THE COMMAND CARRIES NOTHING AND THAT IS THE POINT. Who is at the table and
// which characters are already on it are room state; the hub reads both when it
// resolves this, so there is no list from a browser to be trusted or to have
// gone stale between the page loading and the item being pressed.
//
// A 204 AND NO BODY IS THE WHOLE REPLY. The pawns arrive over the socket as
// pawn.spawned, which is the same way they would arrive for anybody else in the
// room, so there is nothing for this response to swap and nothing for it to
// say. A refusal is the alert modal, out of rejectCommand.
func (a *App) SpawnParty(w http.ResponseWriter, r *http.Request) {
	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}

	if err := a.Hub.Dispatch(r.Context(), roomID, who, &room.PawnSpawnCharacters{}); err != nil {
		a.rejectCommand(w, "spawn the party", err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RoomPawnFragment is the live panel that goes in a window.
func (a *App) RoomPawnFragment(w http.ResponseWriter, r *http.Request) {
	data, ok := a.pawnPanel(w, r, nil)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomPawnFragment(data))
}

// pawnPanel reads the pawn PROJECTED FOR THE ASKER and turns it into the
// panel's data. A false return has already answered with an empty 404, which is
// the only thing a player asking about a pawn they are shown nothing of is ever
// told.
func (a *App) pawnPanel(w http.ResponseWriter, r *http.Request, problems []string) (pages.RoomPawnData, bool) {
	ctx := r.Context()

	row, role, pawn, ok := a.livePawn(ctx, r, r.URL.Query().Get("room"), r.URL.Query().Get("pawn"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)

		return pages.RoomPawnData{}, false
	}

	view, _ := a.Hub.Table(ctx, row.ID)

	return pages.RoomPawnData{
		RoomID:  row.ID.String(),
		IsGM:    role == room.RoleGM,
		CanEdit: mayEditPawn(role, session.FromContext(ctx).UserID, pawn),
		Pawn:    pawnView(pawn, role, layerName(view, pawn.LayerID)),
		Errors:  problems,
	}, true
}

// RoomPawnEditFragment is the form that goes in the content modal.
func (a *App) RoomPawnEditFragment(w http.ResponseWriter, r *http.Request) {
	data, ok := a.pawnForm(w, r, nil)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomPawnForm(data))
}

// pawnForm is the edit form's data. Unlike the panel it is refused outright for
// somebody who may not edit: a form drawn for a reader is a form whose Save
// button is a lie.
func (a *App) pawnForm(w http.ResponseWriter, r *http.Request, problems []string) (pages.RoomPawnFormData, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	row, role, pawn, ok := a.livePawn(ctx, r, r.URL.Query().Get("room"), r.URL.Query().Get("pawn"))
	if !ok || !mayEditPawn(role, sess.UserID, pawn) {
		w.WriteHeader(http.StatusNotFound)

		return pages.RoomPawnFormData{}, false
	}

	data := pages.RoomPawnFormData{
		RoomID:  row.ID.String(),
		IsGM:    role == room.RoleGM,
		Pawn:    pawnView(pawn, role, ""),
		LayerID: pawn.LayerID.String(),
		Shown:   pawn.Visible,
		Errors:  problems,
	}

	// THE LAYER SELECT IS THE GM'S AND SO IS THE READ BEHIND IT. hub.Table is
	// the room's whole configuration, which a player has no business being
	// handed -- the layer manager is refused to them for the same reason.
	if data.IsGM {
		if view, live := a.Hub.Table(ctx, row.ID); live {
			for _, l := range view.Table.Layers {
				data.Layers = append(data.Layers, pages.RoomPawnLayer{
					ID:   l.ID.String(),
					Name: pages.SafeLayerName(l.Name),
				})
			}
		}
	}

	return data, true
}

// RoomConditionRowFragment is one empty condition row, for the form's Add
// button. It is a fragment rather than a clone in JavaScript because the row's
// markup then exists once -- and because server/public/js is not a Tailwind
// source, so a row built there would render with no styling at all.
func (a *App) RoomConditionRowFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	_, role, pawn, ok := a.livePawn(ctx, r, r.URL.Query().Get("room"), r.URL.Query().Get("pawn"))
	if !ok || !mayEditPawn(role, sess.UserID, pawn) || pawn.Kind == room.PawnObject {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomPawnConditionRow(pages.RoomPawnCondition{
		Color:    string(room.ColorRed),
		Duration: "-1",
		Clear:    "end",
	}))
}

// UpdatePawn is the content modal's form: everything about a pawn except the
// one value that changes every round.
//
// IT IS UP TO FOUR COMMANDS AND THEY GO IN ORDER, because the protocol keeps
// them apart on purpose -- conditions are replaced wholesale, visibility is the
// GM's alone and drives the two-audience transitions, and a layer change moves
// a pawn between floors. The plain fields go first: they are the ones that can
// be refused for a value out of range, and a form that had already flipped the
// visibility before failing would leave the GM with half of what they pressed
// Save for.
func (a *App) UpdatePawn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}

	pawnID, err := ulid.Parse(r.PathValue("pawn"))
	if err != nil {
		htmx.NotFound(w, "pawn")

		return
	}

	pawn, live := a.Hub.Pawn(ctx, roomID, pawnID, who.Role)
	if !live || !mayEditPawn(who.Role, sess.UserID, pawn) {
		htmx.NotFound(w, "pawn")

		return
	}

	update, problems := pawnUpdateForm(r, pawn)
	if len(problems) > 0 {
		a.renderPawnFormErrors(w, r, pawnID, problems)

		return
	}

	if err := a.Hub.Dispatch(ctx, roomID, who, update); err != nil {
		a.refusePawnForm(w, r, pawnID, "change a pawn", err)

		return
	}

	if pawn.Kind != room.PawnObject {
		conditions, bad := pawnConditionsForm(r)
		if bad != "" {
			a.renderPawnFormErrors(w, r, pawnID, []string{bad})

			return
		}

		cmd := &room.PawnSetConditions{ID: pawnID, Conditions: conditions}
		if err := a.Hub.Dispatch(ctx, roomID, who, cmd); err != nil {
			a.refusePawnForm(w, r, pawnID, "change a pawn's conditions", err)

			return
		}
	}

	// THE TWO GM-ONLY COMMANDS ARE SENT ONLY WHEN THEY CHANGED, and that is not
	// an optimisation. Both emit to players -- a pawn appearing or disappearing
	// from their table -- so sending one that changes nothing is an event
	// everybody reduces to no effect, and for the layer it is a pawn.updated to
	// the GM's every open window as well.
	if who.Role == room.RoleGM {
		if shown := r.FormValue("shown") != ""; shown != pawn.Visible {
			cmd := &room.PawnSetVisible{ID: pawnID, Visible: shown}
			if err := a.Hub.Dispatch(ctx, roomID, who, cmd); err != nil {
				a.refusePawnForm(w, r, pawnID, "hide or reveal a pawn", err)

				return
			}
		}

		if layer, err := ulid.Parse(r.FormValue("layer")); err == nil && layer != pawn.LayerID {
			cmd := &room.PawnSetLayer{IDs: []ulid.ULID{pawnID}, Layer: layer}
			if err := a.Hub.Dispatch(ctx, roomID, who, cmd); err != nil {
				a.refusePawnForm(w, r, pawnID, "move a pawn between layers", err)

				return
			}
		}
	}

	htmx.CloseModal(w)
	w.WriteHeader(http.StatusNoContent)
}

// UpdatePawnHP is the panel's one inline control, and it answers with the panel.
//
// THE MUTATION RETURNS WHAT IT CHANGED, which is the case the fragment rules
// name. The socket says the same thing a moment later and every other open
// window follows it, but the person who typed "-7" should not watch their own
// entry sit there until their event comes back round.
//
// IT DOES NOT CLOSE A MODAL. There is no modal open behind this -- it is a
// field in a window -- and sending modal:close would dismiss whatever else the
// GM happened to have open.
func (a *App) UpdatePawnHP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}

	pawnID, err := ulid.Parse(r.PathValue("pawn"))
	if err != nil {
		htmx.NotFound(w, "pawn")

		return
	}

	pawn, live := a.Hub.Pawn(ctx, roomID, pawnID, who.Role)
	if !live || !mayEditPawn(who.Role, sess.UserID, pawn) {
		htmx.NotFound(w, "pawn")

		return
	}

	hp, bad := evaluateHP(r.FormValue("hp"), pawn.HP)
	if bad != "" {
		a.renderPawnPanelErrors(w, r, []string{bad})

		return
	}

	if err := a.Hub.Dispatch(ctx, roomID, who, &room.PawnUpdate{ID: pawnID, HP: &hp}); err != nil {
		var refusal *room.Error
		if errors.As(err, &refusal) && refusal.Code == room.CodeInvalid {
			a.renderPawnPanelErrors(w, r, []string{refusal.Message})

			return
		}

		a.rejectCommand(w, "change a pawn's hit points", err)

		return
	}

	data, ok := a.pawnPanel(w, r, nil)
	if !ok {
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomPawnFragment(data))
}

// MovePawnsToLayer is the GM sending a selection upstairs. It takes a list
// because the canvas overlay sends one, and the pawn dialog sends a list of one
// rather than there being a second route for it.
func (a *App) MovePawnsToLayer(w http.ResponseWriter, r *http.Request) {
	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}

	ids, ok := pawnIDs(w, r)
	if !ok {
		return
	}

	layer, err := ulid.Parse(strings.TrimSpace(r.FormValue("layer")))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	if err := a.Hub.Dispatch(r.Context(), roomID, who, &room.PawnSetLayer{IDs: ids, Layer: layer}); err != nil {
		a.rejectCommand(w, "move pawns between layers", err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RemovePawns takes pawns off the table, from the dialog with one id or from
// the overlay with a whole selection.
//
// ONE COMMAND WITH EVERY ID AND NOT ONE PER PAWN. PawnRemove drops the
// initiative entries as it goes and emits a single tracker update at the end;
// five commands would have every client re-render the turn order five times to
// reach the same answer.
func (a *App) RemovePawns(w http.ResponseWriter, r *http.Request) {
	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}

	ids, ok := pawnIDs(w, r)
	if !ok {
		return
	}

	if err := a.Hub.Dispatch(r.Context(), roomID, who, &room.PawnRemove{IDs: ids}); err != nil {
		a.rejectCommand(w, "remove pawns", err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RoomStatBlockFragment is a monster's page of the manual, opened from a pawn.
//
// IT IS THE GM'S AND NOBODY ELSE'S. The room's monster-health setting exists so
// a table can hide a monster's hit points from its players; a stat block
// carries those, its armour class, its resistances and its legendary actions,
// so serving one to a player would contradict, in a second window, the setting
// the GM chose in the first.
//
// IT RENDERS THE PANEL FRAME AND NOT THE DIALOG ONE. The block goes into a
// window, which is dismissed by the controls on its own title bar, so it ships
// no Close of its own -- see the note on pages.StatBlock.
//
// THE OWNER IS THE ROOM'S AND NOT THE ASKER'S, which is the other half of what
// makes this different from MonsterStatBlockFragment. GetMonster and
// ListMonsterActions are both scoped by owner already, so no new statement is
// needed: what changes is which id goes into the parameter. The pawn is read
// out of the live room first, so the monster reached is one the GM actually put
// on this table rather than any id a request cares to name.
func (a *App) RoomStatBlockFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	row, role, pawn, ok := a.livePawn(ctx, r, r.URL.Query().Get("room"), r.URL.Query().Get("pawn"))
	if !ok || role != room.RoleGM || pawn.MonsterID == nil {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	monster, err := a.Queries.GetMonster(ctx, queries.GetMonsterParams{
		ID:      *pawn.MonsterID,
		OwnerID: row.OwnerID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(http.StatusNotFound)

		return
	}
	if err != nil {
		slog.Error("Failed to load a pawn's monster", "error", err)
		htmx.ServerError(w)

		return
	}

	actions, err := a.Queries.ListMonsterActions(ctx, queries.ListMonsterActionsParams{
		MonsterID: monster.ID,
		OwnerID:   row.OwnerID,
	})
	if err != nil {
		slog.Error("Failed to load a pawn's monster actions", "error", err)
		htmx.ServerError(w)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.MonsterStatBlockPanel(monsterStatBlock(monster, actions, monsterDerived(monster, actions))))
}

// livePawn is the three questions every fragment in this file asks: is this a
// room this session is in, is the room running, and what does THIS ROLE see of
// that pawn.
//
// THE PAWN COMES BACK PROJECTED OR NOT AT ALL. See hub.Pawn: a player asking
// about a hidden pawn, or one on another floor, gets nothing -- which the
// caller answers with an empty 404, the same answer a pawn id that never
// existed gets. The two are indistinguishable on purpose.
func (a *App) livePawn(ctx context.Context, r *http.Request, roomID string, pawnID string) (queries.GetRoomRow, room.Role, *room.Pawn, bool) {
	sess := session.FromContext(ctx)

	row, role, err := a.roomMember(ctx, sess, roomID)
	if err != nil || a.Hub == nil {
		return queries.GetRoomRow{}, "", nil, false
	}

	id, err := ulid.Parse(pawnID)
	if err != nil {
		return queries.GetRoomRow{}, "", nil, false
	}

	pawn, ok := a.Hub.Pawn(ctx, row.ID, id, role)
	if !ok {
		return queries.GetRoomRow{}, "", nil, false
	}

	return row, role, pawn, true
}

// pawnActor is the mutation half of the same check: who is asking, about which
// room. It writes the whole response on failure.
func (a *App) pawnActor(w http.ResponseWriter, r *http.Request) (room.Actor, ulid.ULID, bool) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	if a.Hub == nil {
		htmx.NotFound(w, "room")

		return room.Actor{}, ulid.ULID{}, false
	}

	row, role, err := a.roomMember(ctx, sess, r.PathValue("id"))
	if err != nil {
		htmx.NotFound(w, "room")

		return room.Actor{}, ulid.ULID{}, false
	}

	return room.Actor{ID: sess.UserID, Role: role}, row.ID, true
}

// pawnIDs reads the repeated ids field the two list routes take, bounded by the
// protocol's own selection limit. Anything else is an empty 404: the only thing
// that produces one is a request this server did not write.
//
// ParseForm IS WHAT MAKES ONE READ SERVE BOTH VERBS, and the reason is a
// property of each side. htmx puts hx-vals in the QUERY STRING for GET and
// DELETE and in the BODY for everything else -- `/GET|DELETE/.test(method)` in
// its own source -- so the removal arrives one way and the layer move the
// other. net/http's ParseForm merges the query into r.Form for every method and
// the body only for POST, PUT and PATCH, which is exactly the union of the two.
// Reading r.PostForm here instead would work for the layer move and find
// nothing at all for the removal.
func pawnIDs(w http.ResponseWriter, r *http.Request) ([]ulid.ULID, bool) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusNotFound)

		return nil, false
	}

	// COMMAS AS WELL AS REPEATS, and the commas are what the canvas overlay
	// sends. htmx's hx-vals SETS each key rather than appending it, so an array
	// arrives as one value with the elements joined -- there is no way to make
	// it emit a repeated field. A ULID has no comma in it, so splitting is
	// exact, and the pawn dialog's single id goes through the same path
	// unchanged.
	var raw []string
	for _, value := range r.Form["ids"] {
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				raw = append(raw, part)
			}
		}
	}

	if len(raw) == 0 || len(raw) > room.SelectionMax {
		w.WriteHeader(http.StatusNotFound)

		return nil, false
	}

	ids := make([]ulid.ULID, 0, len(raw))
	for _, value := range raw {
		id, err := ulid.Parse(value)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)

			return nil, false
		}
		ids = append(ids, id)
	}

	return ids, true
}

// pawnView turns the projected pawn into strings, which is the last place
// anything is decided about what a viewer is told.
//
// EVERY WITHHELD VALUE IS ALREADY nil BY THE TIME IT GETS HERE, so the empty
// strings below are a consequence of the projection rather than a second copy
// of it. A player looking at a monster in a band room arrives with HP nil,
// MaxHP nil and HPBand set, and there is nothing in this function that could
// put a number back.
func pawnView(pawn *room.Pawn, role room.Role, layer string) pages.RoomPawn {
	out := pages.RoomPawn{
		ID:     pawn.ID.String(),
		Name:   pawn.Name,
		Image:  pawn.Image,
		Object: pawn.Kind == room.PawnObject,
		HP:     pages.PawnHPText(pawn.HP, pawn.MaxHP),
		Layer:  layer,
	}

	if pawn.HPBand != nil {
		out.Band = pages.PawnBandText(string(*pawn.HPBand))
	}
	if pawn.HP != nil {
		out.HPValue = strconv.Itoa(*pawn.HP)
	}
	if pawn.MaxHP != nil {
		out.MaxHP = strconv.Itoa(*pawn.MaxHP)
	}
	if pawn.AC != nil {
		out.AC = strconv.Itoa(*pawn.AC)
	}

	if out.Object {
		out.Pixels = pages.PawnPixelsText(pawn.Width, pawn.Height, pawn.Rotation)
		out.Width = strconv.Itoa(pawn.Width)
		out.Height = strconv.Itoa(pawn.Height)
		out.Rotation = strconv.Itoa(pawn.Rotation)
	} else {
		out.Size = pages.PawnSizeText(string(pawn.Size))
		out.SizeValue = string(pawn.Size)
	}

	for _, c := range pawn.Conditions {
		out.Conditions = append(out.Conditions, pages.RoomPawnCondition{
			ID:           c.ID.String(),
			Name:         c.Name,
			Color:        string(c.Color),
			Duration:     pages.PawnDurationValue(c.Duration),
			DurationText: pages.PawnDurationText(c.Duration),
			Clear:        string(c.Clear),
		})
	}

	// THE TWO GM-ONLY FIELDS, and they are set here rather than in the markup
	// because a template that decided them would be a second place the rule
	// lives. A player never receives a hidden pawn at all, so the first is
	// always false on their copy anyway; the second is what draws the stat
	// block button, which is the GM's alone.
	if role == room.RoleGM {
		out.Hidden = !pawn.Visible
		if pawn.MonsterID != nil {
			out.MonsterID = pawn.MonsterID.String()
		}
	}

	return out
}

// layerName is the floor a pawn stands on, which is worth showing because a
// pawn's window outlives the GM's view of its floor. An unavailable table is an
// empty string rather than a guess -- the panel simply omits the line.
func layerName(view *hub.TableView, id ulid.ULID) string {
	if view == nil {
		return ""
	}

	for _, l := range view.Table.Layers {
		if l.ID == id {
			return pages.SafeLayerName(l.Name)
		}
	}

	return ""
}

// mayEditPawn is the courtesy that decides which controls are drawn, and it is
// the same rule PawnUpdate.Authorize applies again on every post: the GM, or
// the player the pawn belongs to.
func mayEditPawn(role room.Role, user ulid.ULID, pawn *room.Pawn) bool {
	if pawn == nil {
		return false
	}
	if role == room.RoleGM {
		return true
	}

	return pawn.OwnerID != nil && *pawn.OwnerID == user
}

// evaluateHP is the arithmetic the panel's one field takes: 12 sets, -7
// subtracts, +3 adds.
//
// THE SIGN IS THE OPERATOR AND THE ABSENCE OF ONE IS ALSO A DECISION. "7" in a
// box showing 12 means seven, not nineteen; a GM setting a monster's hit points
// to a number reads it off a sheet, and a GM applying damage types the minus
// sign they would say out loud. Clamping is left to the core, which does it
// against the max hit points it holds rather than the ones this happens to have
// been handed.
func evaluateHP(entry string, current *int) (int, string) {
	text := strings.TrimSpace(entry)
	if text == "" {
		return 0, "Enter a number, or a signed change such as -7."
	}

	value, err := strconv.Atoi(text)
	if err != nil {
		return 0, "Enter a number, or a signed change such as -7."
	}

	if text[0] != '+' && text[0] != '-' {
		return value, ""
	}

	from := 0
	if current != nil {
		from = *current
	}

	return from + value, ""
}

// pawnUpdateForm turns the modal's form into the plain-field command. Every
// field is a pointer, so a value the form did not carry is left alone rather
// than reset -- which is what lets the object form omit a creature size and the
// creature form omit a width and a height.
func pawnUpdateForm(r *http.Request, pawn *room.Pawn) (*room.PawnUpdate, []string) {
	var problems []string

	cmd := &room.PawnUpdate{ID: pawn.ID}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		problems = append(problems, "A pawn needs a name.")
	} else {
		cmd.Name = &name
	}

	if value, ok, bad := optionalNumber(r.FormValue("maxHp"), "Maximum hit points"); bad != "" {
		problems = append(problems, bad)
	} else if ok {
		cmd.MaxHP = &value
	}

	if value, ok, bad := optionalNumber(r.FormValue("ac"), "Armour class"); bad != "" {
		problems = append(problems, bad)
	} else if ok {
		cmd.AC = &value
	}

	if pawn.Kind == room.PawnObject {
		width, wBad := requiredNumber(r.FormValue("width"), "Width")
		height, hBad := requiredNumber(r.FormValue("height"), "Height")
		if wBad != "" {
			problems = append(problems, wBad)
		}
		if hBad != "" {
			problems = append(problems, hBad)
		}
		if wBad == "" && hBad == "" {
			cmd.Width, cmd.Height = &width, &height
		}

		// THE ANGLE IS FOLDED RATHER THAN REFUSED, so a GM who types 400 gets a
		// wagon at 40 degrees rather than a form back with a complaint about a
		// number that means exactly what they wanted. The input's own min and
		// max keep an ordinary entry inside one turn; this is what happens when
		// somebody goes round the input.
		if rotation, bad := requiredNumber(r.FormValue("rotation"), "Angle"); bad != "" {
			problems = append(problems, bad)
		} else {
			cmd.Rotation = &rotation
		}

		return cmd, problems
	}

	size := room.Size(strings.TrimSpace(r.FormValue("size")))
	if !size.Valid() {
		problems = append(problems, "Pick one of the six creature sizes.")
	} else {
		cmd.Size = &size
	}

	return cmd, problems
}

// pawnConditionsForm reads the repeater's parallel fields. Every row emits all
// four, so the four slices line up by index; a row whose name was left blank is
// somebody who added one and changed their mind, and is dropped rather than
// refused.
func pawnConditionsForm(r *http.Request) ([]room.Condition, string) {
	ids := r.Form["conditionId"]
	names := r.Form["conditionName"]
	colors := r.Form["conditionColor"]
	durations := r.Form["conditionDuration"]
	clears := r.Form["conditionClear"]

	if len(names) != len(colors) || len(names) != len(durations) || len(names) != len(clears) || len(names) != len(ids) {
		return nil, "That form could not be read. Close it and try again."
	}

	out := make([]room.Condition, 0, len(names))
	for i, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		duration, err := strconv.Atoi(strings.TrimSpace(durations[i]))
		if err != nil {
			return nil, "A condition's turns must be a number, or -1 until it is removed."
		}

		condition := room.Condition{
			Name:     name,
			Color:    room.ConditionColor(colors[i]),
			Duration: duration,
			Clear:    room.ClearTrigger(clears[i]),
		}

		// AN EMPTY ID IS A CHIP SOMEBODY HAS JUST INVENTED and the server mints
		// one. A row that already had an id keeps it, so a duration ticking
		// down does not look like a different condition every round.
		if raw := strings.TrimSpace(ids[i]); raw != "" {
			id, err := ulid.Parse(raw)
			if err != nil {
				return nil, "That form could not be read. Close it and try again."
			}
			condition.ID = id
		}

		out = append(out, condition)
	}

	return out, ""
}

// optionalNumber is a field that may be left empty, which is how a pawn with no
// armour class stays that way.
func optionalNumber(entry string, what string) (int, bool, string) {
	text := strings.TrimSpace(entry)
	if text == "" {
		return 0, false, ""
	}

	value, err := strconv.Atoi(text)
	if err != nil {
		return 0, false, what + " must be a number."
	}

	return value, true, ""
}

func requiredNumber(entry string, what string) (int, string) {
	value, err := strconv.Atoi(strings.TrimSpace(entry))
	if err != nil {
		return 0, what + " must be a number."
	}

	return value, ""
}

// renderPawnFormErrors and renderPawnPanelErrors put a refusal above the field
// that caused it rather than in the alert modal.
//
// THE 422 IS DELIBERATE AND SO IS THE hx-status:422 BESIDE IT. The page's
// noSwap config swallows every 4xx, which is right for a mutation whose answer
// is a dialog; a form with fields needs its errors on screen, so both forms
// carry an override naming their own error slot. It is the shape the character
// panels and the grid form already have.
func (a *App) renderPawnFormErrors(w http.ResponseWriter, r *http.Request, pawnID ulid.ULID, problems []string) {
	renderPanelBlock(w, r, "pawn-form-"+pawnID.String(), problems)
}

func (a *App) renderPawnPanelErrors(w http.ResponseWriter, r *http.Request, problems []string) {
	renderPanelBlock(w, r, pages.RoomPawnPanel+"-"+r.PathValue("pawn"), problems)
}

// refusePawnForm turns a refusal from the protocol into a form error where the
// protocol is complaining about a value, and into the alert modal where it is
// complaining about anything else -- a pawn that is gone, an actor who may not.
func (a *App) refusePawnForm(w http.ResponseWriter, r *http.Request, pawnID ulid.ULID, action string, err error) {
	var refusal *room.Error
	if errors.As(err, &refusal) && refusal.Code == room.CodeInvalid {
		a.renderPawnFormErrors(w, r, pawnID, []string{refusal.Message})

		return
	}

	a.rejectCommand(w, action, err)
}
