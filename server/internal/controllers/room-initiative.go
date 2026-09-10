package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"tabletopper/internal/htmx"
	"tabletopper/internal/hub"
	"tabletopper/internal/room"
	"tabletopper/internal/session"
	"tabletopper/templ/pages"

	"github.com/oklog/ulid/v2"
)

// THE TURN ORDER, AND IT IS NINE ROUTES OVER EIGHT COMMANDS.
//
// EVERY GESTURE IS ONE COMMAND AND ONE TRIP INTO THE ROOM. A drop is
// initiative.reorder, a click is initiative.activate, the x on a line is
// initiative.remove and the Add entry dialog is initiative.add; each of them
// carries what the request said and nothing read back from the room first, so
// a second GM tab cannot slip between a read and a write, and the rules --
// which line is the successor, where a reinforcement goes -- are written once
// in internal/room beside the sync that already had them. initiative.set is
// still there for the editor, which really does replace the whole thing.
//
// EVERY MUTATION ANSWERS 204 AND REDRAWS NOTHING. Each of them ends in
// initiative.updated, and the strip refetches from that -- so the tab that sent
// the command and the one open beside it are corrected by the same event, from
// the same source, at the same instant. A reply carrying markup would correct
// one of them.

// RoomInitiativeFragment is the strip over the table, for anybody in the room.
//
// IT IS THE ONE LIVE SURFACE ON THAT PAGE A PLAYER FETCHES ABOUT THE FIGHT, and
// hub.Initiative answers it with the copy their role may see: the entries of
// hidden pawns are gone, a hidden member is missing from its group's dots, and
// a monster's armour class was never in the response. What a player CAN read
// out of it is a monster's hit points, which is deliberate and is settled in
// projectPawn -- the canvas draws blood from that number, so it has to be in
// the browser for the table to look right.
func (a *App) RoomInitiativeFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	row, role, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil || a.Hub == nil {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	view, ok := a.Hub.Initiative(ctx, row.ID, role)
	if !ok {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomInitiative(initiativeData(row.ID, role, sess.UserID, view)))
}

// RoomInitiativeRoundFragment is the round counter in the menu bar, which
// everybody in the room fetches.
//
// IT IS A SECOND FETCH OF THE SAME TRACKER AND NOT A FIELD ON THE FIRST. The
// counter sits at the right-hand end of the room bar and the strip sits over
// the table, which are two places in the document with a whole page between
// them; htmx swaps one element per response, so a fragment that carried both
// would have to be an out-of-band swap -- a second swap semantics on this page
// for one number. Two GETs of a tracker already in memory is the cheaper half
// of that trade.
//
// IT ANSWERS EVERY ROLE THE SAME. The round is the one thing about a fight
// that is not projected: a player who can see none of the monsters still knows
// which round it is, because their own character is in it.
func (a *App) RoomInitiativeRoundFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	row, role, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil || a.Hub == nil {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	view, ok := a.Hub.Initiative(ctx, row.ID, role)
	if !ok {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// AN EMPTY TRACKER IS AN EMPTY COUNTER, and the counter's own rule is that
	// an empty string prints nothing: a room that is not in a fight is every
	// room most of the time, and a bar permanently reading "Round --" is a
	// label for a thing that is not happening.
	//
	// THERE IS NO "BUILT BUT NOT STARTED" TO PRINT A DASH FOR. A tracker with
	// lines in it is in round one from the moment it is built -- see
	// State.Normalize -- so this is a number or it is nothing.
	round := ""
	if len(view.Initiative.Entries) > 0 {
		round = strconv.Itoa(view.Initiative.Round)
	}

	render(w, r, pages.RoomInitiativeRound(pages.RoomInitiativeRoundData{
		RoomID:  row.ID.String(),
		Round:   round,
		Fetched: true,
	}))
}

// RoomInitiativeEntryFragment is the Add entry dialog, which is the GM's.
func (a *App) RoomInitiativeEntryFragment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess := session.FromContext(ctx)

	row, role, err := a.roomMember(ctx, sess, r.URL.Query().Get("room"))
	if err != nil || role != room.RoleGM {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(w, r, pages.RoomInitiativeEntryForm(pages.RoomInitiativeEntryData{RoomID: row.ID.String()}))
}

// SyncInitiative builds the order from the table, and builds it again to bring
// in reinforcements and take out the corpses.
func (a *App) SyncInitiative(w http.ResponseWriter, r *http.Request) {
	a.initiativeCommand(w, r, "sync the initiative tracker",
		func() (room.Command, bool) { return &room.InitiativeSync{}, true })
}

// NextInitiative advances the turn, and it is the one route here that is not
// the GM's alone: the core lets whoever owns a pawn in the acting line end it.
func (a *App) NextInitiative(w http.ResponseWriter, r *http.Request) {
	a.initiativeCommand(w, r, "advance the turn",
		func() (room.Command, bool) { return &room.InitiativeNext{}, true })
}

// ClearInitiative empties the tracker, behind the confirm modal.
//
// IT IS A POST AND NOT A DELETE, for ClearTabletop's reason: it is a menu item,
// a menu item is a button carrying hx-post, and making this one the exception
// would mean a second branch in RoomMenuItem's markup for one route.
func (a *App) ClearInitiative(w http.ResponseWriter, r *http.Request) {
	a.initiativeCommand(w, r, "clear the initiative tracker",
		func() (room.Command, bool) { return &room.InitiativeClear{}, true })
}

// OrderInitiative is where a drop lands: the whole order, as ids, in the order
// the GM dragged them into. The core refuses a set of ids that is not exactly
// the tracker's, which is a drag that raced a change.
func (a *App) OrderInitiative(w http.ResponseWriter, r *http.Request) {
	a.initiativeCommand(w, r, "reorder the initiative tracker", func() (room.Command, bool) {
		ids, ok := entryIDs(r)
		if !ok {
			return nil, false
		}

		return &room.InitiativeReorder{IDs: ids}, true
	})
}

// ActivateInitiative gives the turn to one line.
func (a *App) ActivateInitiative(w http.ResponseWriter, r *http.Request) {
	a.initiativeCommand(w, r, "change whose turn it is", func() (room.Command, bool) {
		entry, err := ulid.Parse(r.PathValue("entry"))
		if err != nil {
			return nil, false
		}

		return &room.InitiativeActivate{Entry: entry}, true
	})
}

// RemoveInitiative takes one line out.
//
// IT IS NOT CONFIRMED, and that is deliberate: it is undone by pressing Sync
// tracker, which is one menu away. Clear tracker keeps its confirmation,
// because Clear is the whole fight.
func (a *App) RemoveInitiative(w http.ResponseWriter, r *http.Request) {
	a.initiativeCommand(w, r, "remove an entry from the initiative tracker", func() (room.Command, bool) {
		entry, err := ulid.Parse(r.PathValue("entry"))
		if err != nil {
			return nil, false
		}

		return &room.InitiativeRemove{Entry: entry}, true
	})
}

// AddInitiative appends one line: either a name, from the Add entry dialog, or
// a pawn, from that pawn's own menu.
//
// IT TAKES ONE OR THE OTHER AND NEVER BOTH, and the core refuses a request
// carrying both; this route answers the empty dialog with the dialog's own
// error block, because that one is a person who pressed Add too soon rather
// than a request this server did not write.
func (a *App) AddInitiative(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	pawnText := strings.TrimSpace(r.FormValue("pawn"))

	if name == "" && pawnText == "" {
		renderPanelBlock(w, r, pages.RoomInitiativeEntryPanel, []string{"An entry needs a name."})

		return
	}

	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}

	cmd := &room.InitiativeAdd{Name: name}
	if pawnText != "" {
		pawn, err := ulid.Parse(pawnText)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)

			return
		}
		cmd.Pawn = &pawn
	}

	if err := a.Hub.Dispatch(ctx, roomID, who, cmd); err != nil {
		var refusal *room.Error
		if pawnText == "" && errors.As(err, &refusal) && refusal.Code == room.CodeInvalid {
			renderPanelBlock(w, r, pages.RoomInitiativeEntryPanel, []string{refusal.Message})

			return
		}

		a.rejectCommand(w, "add an entry to the initiative tracker", err)

		return
	}

	// THE DIALOG CLOSES ITSELF AND THE STRIP REDRAWS FROM THE EVENT. There is
	// nothing to swap: the line that was just added arrives on every screen in
	// the room, this one included, over the socket.
	if pawnText == "" {
		htmx.CloseModal(w)
	}

	w.WriteHeader(http.StatusNoContent)
}

// initiativeCommand is the shape every mutation in this file has: establish the
// asker and the room, build the command from the request, send it, and answer
// 204 or the refusal. It is layerCommand's shape, one file over.
//
// build answers false for a request this server did not write -- an entry id
// that is not an id, an order with nothing in it. That is a 404 rather than a
// message, because the only thing that produces one is somebody posting by
// hand.
func (a *App) initiativeCommand(w http.ResponseWriter, r *http.Request, action string, build func() (room.Command, bool)) {
	ctx := r.Context()

	who, roomID, ok := a.pawnActor(w, r)
	if !ok {
		return
	}

	cmd, ok := build()
	if !ok {
		w.WriteHeader(http.StatusNotFound)

		return
	}

	if err := a.Hub.Dispatch(ctx, roomID, who, cmd); err != nil {
		a.rejectCommand(w, action, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// initiativeData turns the projected tracker into the strip, and it is the last
// place anything is decided about what a viewer is told.
//
// EVERY WITHHELD VALUE IS ALREADY GONE BY THE TIME IT GETS HERE. hub.Initiative
// answers with the copy this role may see -- hidden lines dropped, hidden
// members missing from their group, armour class never sent -- so nothing below
// could put any of it back. What this function still decides is the TEXT, which
// is what the room's label setting governs: see ExactHP.
func initiativeData(roomID ulid.ULID, role room.Role, user ulid.ULID, view *hub.InitiativeView) pages.RoomInitiativeData {
	data := pages.RoomInitiativeData{
		RoomID: roomID.String(),
		IsGM:   role == room.RoleGM,
		Empty:  len(view.Initiative.Entries) == 0,
	}

	for _, e := range view.Initiative.Entries {
		data.Entries = append(data.Entries, initiativeEntryData(role, user, view, e))
	}

	return data
}

// initiativeEntryData is one line: which of the three shapes it is, whose face
// it wears, and what the person looking at it may read off it.
func initiativeEntryData(role room.Role, user ulid.ULID, view *hub.InitiativeView, e room.InitiativeEntry) pages.RoomInitiativeEntry {
	out := pages.RoomInitiativeEntry{
		ID:     e.ID.String(),
		Name:   e.Name,
		Kind:   pages.EntryNamed,
		Active: view.Initiative.Active != nil && *view.Initiative.Active == e.ID,
	}

	members := make([]room.Pawn, 0, len(e.PawnIDs))
	for _, id := range e.PawnIDs {
		if p, found := view.Pawns[id]; found {
			members = append(members, p)
		}
	}

	// A LINE WITH NO CREATURE IS DRAWN FROM ITS OWN NAME, and that is the only
	// thing the entry's name is for. Everything else prints its pawns' CURRENT
	// names, so a goblin renamed mid-fight reads the same on the strip as on
	// the table.
	if len(members) == 0 {
		return out
	}

	out.Name = members[0].Name
	out.Kind = pages.EntrySolo
	if len(members) > 1 {
		out.Kind = pages.EntryGroup
	}

	// WHAT COLOURS THE FRAME, and it is read off the first member because a
	// group is only ever monsters: every member of one got there by having the
	// same monster key. The words are room.PawnKind's own, so there is no
	// table between the protocol and the attribute the stylesheet keys on.
	out.Side = string(members[0].Kind)

	for _, p := range members {
		if p.OwnerID != nil && *p.OwnerID == user {
			out.Mine = true

			break
		}
	}

	if role == room.RoleGM {
		for _, p := range members {
			if !p.Visible {
				out.Hidden = true

				break
			}
		}
	}

	// THE PORTRAIT IS THE WORST-HURT MEMBER STILL STANDING, so a group in
	// trouble looks like it, and a group whose every member is dead takes the
	// corpse treatment whole.
	face := worstHurt(members)
	out.Image = face.Image
	out.Blood = pages.InitiativeBloodVariant(face.ID.String())
	if band := room.Health(face); band != nil {
		out.Band = string(*band)
	}

	if out.Kind == pages.EntryGroup {
		bands := make([]string, 0, len(members))
		for _, p := range members {
			band := room.Health(p)
			if band == nil {
				bands = append(bands, "")

				continue
			}
			bands = append(bands, string(*band))
		}
		out.Pips, out.Count = pages.InitiativePips(bands)

		// A GROUP CARRIES NO HIT-POINT TEXT AND NO CONDITION CHIPS. Nine
		// goblins have nine of each; the dots say how the group is doing and
		// the rings on the table are where the rest of it lives.
		return out
	}

	solo := members[0]
	out.Solo = solo.ID.String()

	if room.ExactHP(solo.Kind, view.Table.PawnLabels, role) {
		out.HP = pages.PawnHPText(solo.HP, solo.MaxHP)
	} else if solo.HPBand != nil {
		out.HP = pages.PawnBandText(string(*solo.HPBand))
	}

	out.Conditions = pawnConditions(solo.Conditions)

	return out
}

// worstHurt is whose face a group wears: the most badly injured member that is
// still alive, because a group in trouble should look like it and a corpse's
// portrait would say the whole group had fallen. When every member IS dead, the
// first of them is the face and the line goes grey under a skull.
func worstHurt(members []room.Pawn) room.Pawn {
	face := members[0]
	worst := -1

	for _, p := range members {
		rank := bandRank(room.Health(p))
		if rank == deadRank {
			continue
		}
		if rank > worst {
			worst = rank
			face = p
		}
	}

	return face
}

// deadRank is the top of the scale below, and it is named because worstHurt
// compares against it rather than against the word.
const deadRank = 6

// bandRank orders the six bands by how bad they are, so that "the worst of
// these" is a comparison rather than five cases. A creature nobody told this
// viewer anything about ranks lowest: it is not evidence that the group is in
// trouble.
func bandRank(band *room.HPBand) int {
	if band == nil {
		return 0
	}

	switch *band {
	case room.BandHealthy:
		return 1
	case room.BandBruised:
		return 2
	case room.BandBloody:
		return 3
	case room.BandVeryBloody:
		return 4
	case room.BandNearDeath:
		return 5
	case room.BandDead:
		return deadRank
	}

	return 0
}

// entryIDs reads the order a drop posted: one comma-separated field, which is
// what hx-vals can express -- it SETS each key rather than appending it, so an
// array arrives as one value with the elements joined. A ULID has no comma in
// it, so splitting is exact. It is pawnIDs one file over, for the same reason.
func entryIDs(r *http.Request) ([]ulid.ULID, bool) {
	if err := r.ParseForm(); err != nil {
		return nil, false
	}

	var raw []string
	for _, value := range r.Form["entries"] {
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				raw = append(raw, part)
			}
		}
	}

	if len(raw) == 0 || len(raw) > room.InitiativeMax {
		return nil, false
	}

	ids := make([]ulid.ULID, 0, len(raw))
	for _, value := range raw {
		id, err := ulid.Parse(value)
		if err != nil {
			return nil, false
		}
		ids = append(ids, id)
	}

	return ids, true
}
